package cache

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	schedulingv1alpha1 "github.com/bupt20220407/accelerator-fabric-scheduler/pkg/apis/scheduling/v1alpha1"
	"github.com/bupt20220407/accelerator-fabric-scheduler/pkg/topology"
)

var ErrOlderGeneration = errors.New("topology update is older than the cached generation")

type Availability string

const (
	AvailabilityReady    Availability = "Ready"
	AvailabilityMissing  Availability = "Missing"
	AvailabilityNotReady Availability = "NotReady"
	AvailabilityStale    Availability = "Stale"
)

type Result struct {
	Snapshot           *topology.Snapshot
	Availability       Availability
	ResourceVersion    string
	ObjectGeneration   int64
	ObservedGeneration int64
	LastHeartbeatTime  time.Time
}

type entry struct {
	result Result
}

type storeState struct {
	entries map[string]entry
}

type Store struct {
	mu    sync.Mutex
	state atomic.Pointer[storeState]
}

func NewStore() *Store {
	store := &Store{}
	store.state.Store(&storeState{entries: map[string]entry{}})
	return store
}

func (s *Store) Upsert(object *schedulingv1alpha1.AcceleratorTopology) error {
	if object == nil {
		return fmt.Errorf("topology object is nil")
	}
	snapshot, err := topology.Build(object.Spec)
	if err != nil {
		return err
	}

	result := Result{
		Snapshot:           snapshot,
		Availability:       AvailabilityNotReady,
		ResourceVersion:    object.ResourceVersion,
		ObjectGeneration:   object.Generation,
		ObservedGeneration: object.Spec.ObservedGeneration,
	}
	readyCondition := apiMeta.FindStatusCondition(object.Status.Conditions, "Ready")
	if readyCondition != nil && readyCondition.Status == metav1.ConditionTrue && readyCondition.ObservedGeneration == object.Generation {
		result.Availability = AvailabilityReady
	}
	if object.Status.LastHeartbeatTime != nil {
		result.LastHeartbeatTime = object.Status.LastHeartbeatTime.Time
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.state.Load()
	if cached, found := current.entries[object.Spec.NodeName]; found {
		if object.Generation < cached.result.ObjectGeneration ||
			(object.Generation == cached.result.ObjectGeneration && object.Spec.ObservedGeneration < cached.result.ObservedGeneration) {
			return ErrOlderGeneration
		}
	}

	next := cloneState(current)
	next.entries[object.Spec.NodeName] = entry{result: result}
	s.state.Store(next)
	return nil
}

func (s *Store) Delete(nodeName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.state.Load()
	if _, found := current.entries[nodeName]; !found {
		return
	}
	next := cloneState(current)
	delete(next.entries, nodeName)
	s.state.Store(next)
}

func (s *Store) Get(nodeName string, now time.Time, ttl time.Duration) Result {
	current := s.state.Load()
	cached, found := current.entries[nodeName]
	if !found {
		return Result{Availability: AvailabilityMissing}
	}
	result := cached.result
	if result.Availability != AvailabilityReady {
		return result
	}
	if result.LastHeartbeatTime.IsZero() || (ttl > 0 && now.Sub(result.LastHeartbeatTime) > ttl) {
		result.Availability = AvailabilityStale
	}
	return result
}

func (s *Store) Len() int {
	return len(s.state.Load().entries)
}

func cloneState(current *storeState) *storeState {
	entries := make(map[string]entry, len(current.entries)+1)
	for key, value := range current.entries {
		entries[key] = value
	}
	return &storeState{entries: entries}
}
