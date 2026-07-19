package policy

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	schedulinginformers "github.com/bupt/accelerator-fabric-scheduler/pkg/generated/informers/externalversions/scheduling/v1alpha1"
	schedulinglisters "github.com/bupt/accelerator-fabric-scheduler/pkg/generated/listers/scheduling/v1alpha1"
	"github.com/bupt/accelerator-fabric-scheduler/pkg/observability"
)

type Controller struct {
	kubeClient kubernetes.Interface
	lister     schedulinglisters.AcceleratorPlacementPolicyLister
	synced     toolscache.InformerSynced
	queue      workqueue.TypedRateLimitingInterface[toolscache.ObjectName]
}

func New(kubeClient kubernetes.Interface, informer schedulinginformers.AcceleratorPlacementPolicyInformer) *Controller {
	controller := &Controller{
		kubeClient: kubeClient,
		lister:     informer.Lister(),
		synced:     informer.Informer().HasSynced,
		queue:      workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[toolscache.ObjectName]()),
	}
	_, _ = informer.Informer().AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    controller.enqueue,
		UpdateFunc: func(_, current any) { controller.enqueue(current) },
		DeleteFunc: controller.enqueue,
	})
	return controller
}

func (c *Controller) Run(ctx context.Context, workers int) error {
	defer c.queue.ShutDown()
	if workers < 1 {
		return fmt.Errorf("workers must be positive")
	}
	if !toolscache.WaitForCacheSync(ctx.Done(), c.synced) {
		return fmt.Errorf("wait for placement policy informer cache sync")
	}
	for worker := 0; worker < workers; worker++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}
	<-ctx.Done()
	return nil
}

func (c *Controller) runWorker(ctx context.Context) {
	for c.processNext(ctx) {
	}
}

func (c *Controller) processNext(ctx context.Context) bool {
	objectName, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(objectName)
	started := time.Now()
	if err := c.sync(ctx, objectName); err != nil {
		observability.ObserveControllerReconcile("policy", observability.ResultError, time.Since(started))
		utilruntime.HandleErrorWithContext(ctx, err, "sync accelerator placement policy", "namespace", objectName.Namespace, "name", objectName.Name)
		c.queue.AddRateLimited(objectName)
		return true
	}
	observability.ObserveControllerReconcile("policy", observability.ResultSuccess, time.Since(started))
	c.queue.Forget(objectName)
	return true
}

func (c *Controller) sync(ctx context.Context, objectName toolscache.ObjectName) error {
	policy, err := c.lister.AcceleratorPlacementPolicies(objectName.Namespace).Get(objectName.Name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	templates := c.kubeClient.ResourceV1().ResourceClaimTemplates(policy.Namespace)
	existing, getErr := templates.Get(ctx, policy.Name, metav1.GetOptions{})
	if policy.Spec.DRA == nil {
		if apierrors.IsNotFound(getErr) {
			return nil
		}
		if getErr != nil {
			return getErr
		}
		if existing.Labels[managedByLabel] != managedByValue || !metav1.IsControlledBy(existing, policy) {
			return nil
		}
		return templates.Delete(ctx, existing.Name, metav1.DeleteOptions{})
	}
	desired, err := ResourceClaimTemplateForPolicy(policy)
	if err != nil {
		return err
	}
	if apierrors.IsNotFound(getErr) {
		_, err = templates.Create(ctx, desired, metav1.CreateOptions{FieldManager: managedByValue})
		return err
	}
	if getErr != nil {
		return getErr
	}
	if existing.Labels[managedByLabel] != managedByValue || !metav1.IsControlledBy(existing, policy) {
		return fmt.Errorf("resource claim template %s/%s is not managed by this controller", existing.Namespace, existing.Name)
	}
	if equality.Semantic.DeepEqual(existing.Spec, desired.Spec) &&
		equality.Semantic.DeepEqual(existing.Labels, desired.Labels) &&
		equality.Semantic.DeepEqual(existing.OwnerReferences, desired.OwnerReferences) {
		return nil
	}
	updated := existing.DeepCopy()
	updated.Spec = desired.Spec
	updated.Labels = desired.Labels
	updated.OwnerReferences = desired.OwnerReferences
	_, err = templates.Update(ctx, updated, metav1.UpdateOptions{FieldManager: managedByValue})
	return err
}

func (c *Controller) enqueue(object any) {
	objectName, err := toolscache.ObjectToName(object)
	if err == nil {
		c.queue.Add(objectName)
	}
}
