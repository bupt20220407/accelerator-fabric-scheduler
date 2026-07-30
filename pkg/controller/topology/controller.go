package topology

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	clientset "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/clientset/versioned"
	informers "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/informers/externalversions/scheduling/v1alpha1"
	listers "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/generated/listers/scheduling/v1alpha1"
	"github.com/bupt20220407/accelerator-fabric-scheduler/pkg/observability"
	graph "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/topology"
)

const (
	fieldManager      = "accelerator-topology-controller"
	heartbeatInterval = 30 * time.Second
)

type Controller struct {
	client clientset.Interface
	lister listers.AcceleratorTopologyLister
	synced toolscache.InformerSynced
	queue  workqueue.TypedRateLimitingInterface[toolscache.ObjectName]
	now    func() time.Time
}

func New(client clientset.Interface, informer informers.AcceleratorTopologyInformer) *Controller {
	rateLimiter := workqueue.NewTypedMaxOfRateLimiter(
		workqueue.NewTypedItemExponentialFailureRateLimiter[toolscache.ObjectName](5*time.Millisecond, 30*time.Second),
		&workqueue.TypedBucketRateLimiter[toolscache.ObjectName]{Limiter: rate.NewLimiter(rate.Limit(20), 100)},
	)
	controller := &Controller{
		client: client,
		lister: informer.Lister(),
		synced: informer.Informer().HasSynced,
		queue:  workqueue.NewTypedRateLimitingQueue(rateLimiter),
		now:    time.Now,
	}
	_, _ = informer.Informer().AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    controller.enqueue,
		UpdateFunc: func(_, current any) { controller.enqueue(current) },
	})
	return controller
}

func (c *Controller) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	if workers < 1 {
		return fmt.Errorf("workers must be positive")
	}
	if !toolscache.WaitForCacheSync(ctx.Done(), c.synced) {
		return fmt.Errorf("wait for topology informer cache sync")
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
		observability.ObserveControllerReconcile("topology", observability.ResultError, time.Since(started))
		utilruntime.HandleErrorWithContext(ctx, err, "sync accelerator topology", "name", objectName.Name)
		c.queue.AddRateLimited(objectName)
		return true
	}
	observability.ObserveControllerReconcile("topology", observability.ResultSuccess, time.Since(started))
	c.queue.Forget(objectName)
	return true
}

func (c *Controller) sync(ctx context.Context, objectName toolscache.ObjectName) error {
	object, err := c.lister.Get(objectName.Name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	now := c.now()
	status, reason, message := metav1.ConditionTrue, "ValidTopology", "topology graph passed validation"
	if _, err := graph.Build(object.Spec); err != nil {
		status, reason, message = metav1.ConditionFalse, "InvalidTopology", err.Error()
	}
	current := apiMeta.FindStatusCondition(object.Status.Conditions, "Ready")
	if current != nil && current.Status == status && current.Reason == reason && current.Message == message &&
		current.ObservedGeneration == object.Generation && object.Status.LastHeartbeatTime != nil &&
		now.Sub(object.Status.LastHeartbeatTime.Time) < heartbeatInterval {
		return nil
	}

	updated := object.DeepCopy()
	apiMeta.SetStatusCondition(&updated.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             status,
		ObservedGeneration: object.Generation,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.NewTime(now),
	})
	heartbeat := metav1.NewTime(now)
	updated.Status.LastHeartbeatTime = &heartbeat
	_, err = c.client.SchedulingV1alpha1().AcceleratorTopologies().UpdateStatus(ctx, updated, metav1.UpdateOptions{FieldManager: fieldManager})
	return err
}

func (c *Controller) enqueue(object any) {
	objectName, err := toolscache.ObjectToName(object)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(objectName)
}
