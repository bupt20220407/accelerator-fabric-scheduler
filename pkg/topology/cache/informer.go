package cache

import (
	"fmt"

	toolscache "k8s.io/client-go/tools/cache"

	schedulingv1alpha1 "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	schedulinginformers "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/informers/externalversions/scheduling/v1alpha1"
)

type ErrorHandler func(error)

func ObserveAcceleratorTopologies(
	informer schedulinginformers.AcceleratorTopologyInformer,
	store *Store,
	onError ErrorHandler,
) (toolscache.ResourceEventHandlerRegistration, error) {
	if informer == nil {
		return nil, fmt.Errorf("topology informer is nil")
	}
	if store == nil {
		return nil, fmt.Errorf("topology store is nil")
	}
	if onError == nil {
		onError = func(error) {}
	}

	return informer.Informer().AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(object any) {
			topologyObject, ok := object.(*schedulingv1alpha1.AcceleratorTopology)
			if !ok {
				onError(fmt.Errorf("unexpected topology add object %T", object))
				return
			}
			if err := store.Upsert(topologyObject); err != nil {
				onError(fmt.Errorf("cache topology %q: %w", topologyObject.Name, err))
			}
		},
		UpdateFunc: func(_, newObject any) {
			topologyObject, ok := newObject.(*schedulingv1alpha1.AcceleratorTopology)
			if !ok {
				onError(fmt.Errorf("unexpected topology update object %T", newObject))
				return
			}
			if err := store.Upsert(topologyObject); err != nil {
				onError(fmt.Errorf("update cached topology %q: %w", topologyObject.Name, err))
			}
		},
		DeleteFunc: func(object any) {
			topologyObject, err := topologyFromDelete(object)
			if err != nil {
				onError(err)
				return
			}
			store.Delete(topologyObject.Spec.NodeName)
		},
	})
}

func topologyFromDelete(object any) (*schedulingv1alpha1.AcceleratorTopology, error) {
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
