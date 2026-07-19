package policy

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	schedulingv1alpha1 "github.com/bupt/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/dra"
)

const (
	managedByLabel = "scheduling.bupt.dev/managed-by"
	managedByValue = "accelerator-policy-controller"
	policyLabel    = "scheduling.bupt.dev/placement-policy"
	requestName    = "accelerators"
)

func ResourceClaimTemplateForPolicy(policy *schedulingv1alpha1.AcceleratorPlacementPolicy) (*resourceapi.ResourceClaimTemplate, error) {
	if policy == nil || policy.Spec.DRA == nil {
		return nil, fmt.Errorf("policy with DRA parameters is required")
	}
	if policy.Spec.DRA.DeviceCount < 1 {
		return nil, fmt.Errorf("DRA device count must be positive")
	}
	if policy.Spec.AllowUnknownTopology {
		return nil, fmt.Errorf("DRA template generation requires known topology")
	}

	selectors := []resourceapi.DeviceSelector{celSelector(attribute("health") + ` == "Healthy"`)}
	if policy.Spec.Vendor != "" && policy.Spec.Vendor != schedulingv1alpha1.VendorAny {
		selectors = append(selectors, celSelector(attribute("vendor")+" == "+strconv.Quote(string(policy.Spec.Vendor))))
	}
	if len(policy.Spec.Products) > 0 {
		products := make([]string, 0, len(policy.Spec.Products))
		for _, product := range policy.Spec.Products {
			products = append(products, strconv.Quote(product))
		}
		selectors = append(selectors, celSelector(attribute("product")+" in ["+strings.Join(products, ",")+"]"))
	}
	if policy.Spec.MinimumMemoryMiB > 0 {
		selectors = append(selectors, celSelector(fmt.Sprintf("%s >= %d", attribute("memoryMiB"), policy.Spec.MinimumMemoryMiB)))
	}

	var matchAttribute string
	switch policy.Spec.TopologyMode {
	case schedulingv1alpha1.TopologyModeSingleNUMA:
		matchAttribute = dra.DriverName + "/numaNode"
	case schedulingv1alpha1.TopologyModeSamePCIeRoot:
		matchAttribute = dra.DriverName + "/pcieRoot"
	case schedulingv1alpha1.TopologyModeFabricClique:
		matchAttribute = dra.DriverName + "/fabricGroup"
		if policy.Spec.MinimumLinkBandwidthGBps > 0 {
			selectors = append(selectors, celSelector(fmt.Sprintf("%s >= %d", attribute("fabricBandwidthGBps"), int64(math.Ceil(policy.Spec.MinimumLinkBandwidthGBps)))))
		}
		selectors = append(selectors, celSelector(fmt.Sprintf("%s <= %d", attribute("fabricHops"), policy.Spec.MaxFabricHops)))
	case schedulingv1alpha1.TopologyModeBestEffort:
	case schedulingv1alpha1.TopologyModeFabricConnected:
		return nil, fmt.Errorf("DRA template generation does not support fabric-connected mode")
	default:
		return nil, fmt.Errorf("unsupported topology mode %q", policy.Spec.TopologyMode)
	}

	constraints := []resourceapi.DeviceConstraint(nil)
	if matchAttribute != "" {
		qualified := resourceapi.FullyQualifiedName(matchAttribute)
		constraints = []resourceapi.DeviceConstraint{{Requests: []string{requestName}, MatchAttribute: &qualified}}
	}
	controller, blockOwnerDeletion := true, true
	return &resourceapi.ResourceClaimTemplate{
		TypeMeta: metav1.TypeMeta{APIVersion: resourceapi.SchemeGroupVersion.String(), Kind: "ResourceClaimTemplate"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      policy.Name,
			Namespace: policy.Namespace,
			Labels: map[string]string{
				managedByLabel: managedByValue,
				policyLabel:    policy.Name,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         schedulingv1alpha1.SchemeGroupVersion.String(),
				Kind:               "AcceleratorPlacementPolicy",
				Name:               policy.Name,
				UID:                policy.UID,
				Controller:         &controller,
				BlockOwnerDeletion: &blockOwnerDeletion,
			}},
		},
		Spec: resourceapi.ResourceClaimTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{policyLabel: policy.Name}},
			Spec: resourceapi.ResourceClaimSpec{Devices: resourceapi.DeviceClaim{
				Requests: []resourceapi.DeviceRequest{{
					Name: requestName,
					Exactly: &resourceapi.ExactDeviceRequest{
						DeviceClassName: dra.DeviceClassName,
						Selectors:       selectors,
						AllocationMode:  resourceapi.DeviceAllocationModeExactCount,
						Count:           policy.Spec.DRA.DeviceCount,
					},
				}},
				Constraints: constraints,
			}},
		},
	}, nil
}

func attribute(name string) string {
	return `device.attributes["` + dra.DriverName + `"].` + name
}

func celSelector(expression string) resourceapi.DeviceSelector {
	return resourceapi.DeviceSelector{CEL: &resourceapi.CELDeviceSelector{Expression: expression}}
}
