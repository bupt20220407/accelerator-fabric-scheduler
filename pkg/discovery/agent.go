package discovery

import (
	"context"
	"fmt"
	"reflect"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	schedulingv1alpha1 "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	clientset "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/clientset/versioned"
)

const (
	ManagedByLabel         = "app.kubernetes.io/managed-by"
	ManagedByValue         = "accelerator-topology-discovery"
	ProviderLabel          = "scheduling.bupt.dev/provider"
	DiscoveredAtAnnotation = "scheduling.bupt.dev/discovered-at"
)

type Agent struct {
	client   clientset.Interface
	provider Provider
	nodeName string
	now      func() time.Time
}

func NewAgent(client clientset.Interface, provider Provider, nodeName string) (*Agent, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}
	if provider == nil {
		return nil, fmt.Errorf("provider is required")
	}
	if nodeName == "" {
		return nil, fmt.Errorf("node name is required")
	}
	return &Agent{client: client, provider: provider, nodeName: nodeName, now: time.Now}, nil
}

func (a *Agent) Reconcile(ctx context.Context) error {
	discovered, err := a.provider.Discover(ctx, a.nodeName)
	if err != nil {
		return fmt.Errorf("discover topology: %w", err)
	}
	if discovered.NodeName != a.nodeName {
		return fmt.Errorf("provider returned node %q for agent node %q", discovered.NodeName, a.nodeName)
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		objects := a.client.SchedulingV1alpha1().AcceleratorTopologies()
		current, err := objects.Get(ctx, a.nodeName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			created := &schedulingv1alpha1.AcceleratorTopology{
				TypeMeta:   metav1.TypeMeta{APIVersion: schedulingv1alpha1.SchemeGroupVersion.String(), Kind: "AcceleratorTopology"},
				ObjectMeta: metav1.ObjectMeta{Name: a.nodeName},
				Spec:       discovered,
			}
			created.Spec.ObservedGeneration = 1
			a.stamp(created)
			_, err = objects.Create(ctx, created, metav1.CreateOptions{FieldManager: ManagedByValue})
			return err
		}
		if err != nil {
			return err
		}
		if err := a.validateOwnership(current, discovered); err != nil {
			return err
		}

		updated := current.DeepCopy()
		if topologyChanged(current.Spec, discovered) {
			next := current.Spec.ObservedGeneration + 1
			if next < 1 {
				next = 1
			}
			updated.Spec = discovered
			updated.Spec.ObservedGeneration = next
		}
		a.stamp(updated)
		_, err = objects.Update(ctx, updated, metav1.UpdateOptions{FieldManager: ManagedByValue})
		return err
	})
}

func (a *Agent) validateOwnership(current *schedulingv1alpha1.AcceleratorTopology, discovered schedulingv1alpha1.AcceleratorTopologySpec) error {
	manager := current.Labels[ManagedByLabel]
	provider := current.Labels[ProviderLabel]
	if manager != "" && manager != ManagedByValue {
		return fmt.Errorf("topology %q is managed by %q", current.Name, manager)
	}
	if provider != "" && provider != a.provider.Name() {
		return fmt.Errorf("topology %q belongs to provider %q", current.Name, provider)
	}
	if manager == "" && !specContentEqual(current.Spec, discovered) {
		return fmt.Errorf("refusing to adopt unmanaged topology %q with different content", current.Name)
	}
	return nil
}

func (a *Agent) stamp(object *schedulingv1alpha1.AcceleratorTopology) {
	if object.Labels == nil {
		object.Labels = map[string]string{}
	}
	if object.Annotations == nil {
		object.Annotations = map[string]string{}
	}
	object.Labels[ManagedByLabel] = ManagedByValue
	object.Labels[ProviderLabel] = a.provider.Name()
	object.Annotations[DiscoveredAtAnnotation] = a.now().UTC().Format(time.RFC3339Nano)
}

func topologyChanged(current, discovered schedulingv1alpha1.AcceleratorTopologySpec) bool {
	return !specContentEqual(current, discovered)
}

func specContentEqual(left, right schedulingv1alpha1.AcceleratorTopologySpec) bool {
	left.ObservedGeneration = 0
	right.ObservedGeneration = 0
	return reflect.DeepEqual(left, right)
}
