package dra

import (
	"context"
	"fmt"
	"time"

	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/dynamic-resource-allocation/resourceslice"

	schedulingv1alpha1 "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	schedulinginformers "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/informers/externalversions/scheduling/v1alpha1"
	"github.com/bupt20220407/accelerator-fabric-scheduler/pkg/observability"
)

const topologyTTL = 2 * time.Minute

type PublishResourcesFunc func(context.Context, resourceslice.DriverResources) error

func ObserveTopologies(
	ctx context.Context,
	informer schedulinginformers.AcceleratorTopologyInformer,
	nodeName string,
	publish PublishResourcesFunc,
	now func() time.Time,
	onError func(error),
) (toolscache.ResourceEventHandlerRegistration, error) {
	if informer == nil || nodeName == "" || publish == nil {
		return nil, fmt.Errorf("topology informer, node name, and publisher are required")
	}
	if now == nil {
		now = time.Now
	}
	if onError == nil {
		onError = func(error) {}
	}
	publishObserved := func(resources resourceslice.DriverResources) error {
		started := time.Now()
		err := publish(ctx, resources)
		result := observability.ResultSuccess
		if err != nil {
			result = observability.ResultError
		} else {
			observability.SetDRAPublishedDevices(publishedDeviceCount(resources))
		}
		observability.ObserveDRAOperation("publish", result, time.Since(started))
		return err
	}
	publishObject := func(object *schedulingv1alpha1.AcceleratorTopology) {
		if object.Spec.NodeName != nodeName {
			return
		}
		resources := EmptyResources()
		if topologyReady(object, now()) {
			converted, err := ResourcesForTopology(object)
			if err != nil {
				onError(fmt.Errorf("convert topology %q: %w", object.Name, err))
				return
			}
			resources = converted
		}
		if err := publishObserved(resources); err != nil {
			onError(fmt.Errorf("publish topology %q: %w", object.Name, err))
		}
	}
	return informer.Informer().AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(object any) {
			if topologyObject, ok := object.(*schedulingv1alpha1.AcceleratorTopology); ok {
				publishObject(topologyObject)
			}
		},
		UpdateFunc: func(oldObject, newObject any) {
			oldTopology, oldOK := oldObject.(*schedulingv1alpha1.AcceleratorTopology)
			newTopology, newOK := newObject.(*schedulingv1alpha1.AcceleratorTopology)
			if oldOK && oldTopology.Spec.NodeName == nodeName && (!newOK || newTopology.Spec.NodeName != nodeName) {
				if err := publishObserved(EmptyResources()); err != nil {
					onError(err)
				}
				return
			}
			if newOK {
				publishObject(newTopology)
			}
		},
		DeleteFunc: func(object any) {
			topologyObject, err := deletedTopology(object)
			if err != nil {
				onError(err)
				return
			}
			if topologyObject.Spec.NodeName == nodeName {
				if err := publishObserved(EmptyResources()); err != nil {
					onError(err)
				}
			}
		},
	})
}

func publishedDeviceCount(resources resourceslice.DriverResources) int {
	count := 0
	for _, pool := range resources.Pools {
		for _, slice := range pool.Slices {
			count += len(slice.Devices)
		}
	}
	return count
}

func topologyReady(object *schedulingv1alpha1.AcceleratorTopology, now time.Time) bool {
	condition := apiMeta.FindStatusCondition(object.Status.Conditions, "Ready")
	return condition != nil && condition.Status == metav1.ConditionTrue &&
		condition.ObservedGeneration == object.Generation &&
		object.Status.LastHeartbeatTime != nil &&
		now.Sub(object.Status.LastHeartbeatTime.Time) <= topologyTTL
}

func deletedTopology(object any) (*schedulingv1alpha1.AcceleratorTopology, error) {
	if topologyObject, ok := object.(*schedulingv1alpha1.AcceleratorTopology); ok {
		return topologyObject, nil
	}
	tombstone, ok := object.(toolscache.DeletedFinalStateUnknown)
	if !ok {
		return nil, fmt.Errorf("unexpected topology delete object %T", object)
	}
	topologyObject, ok := tombstone.Obj.(*schedulingv1alpha1.AcceleratorTopology)
	if !ok {
		return nil, fmt.Errorf("unexpected topology tombstone object %T", tombstone.Obj)
	}
	return topologyObject, nil
}
