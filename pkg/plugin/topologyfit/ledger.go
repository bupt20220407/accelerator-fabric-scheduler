package topologyfit

import (
	"errors"
	"fmt"
	"sync"
)

var ErrNoAvailableCombination = errors.New("no unreserved device combination is available")

type reservation struct {
	podKey    string
	nodeName  string
	deviceIDs []string
}

type reservationLedger struct {
	mu          sync.Mutex
	byPod       map[string]reservation
	deviceOwner map[string]map[string]string
}

func newReservationLedger() *reservationLedger {
	return &reservationLedger{
		byPod:       make(map[string]reservation),
		deviceOwner: make(map[string]map[string]string),
	}
}

func (l *reservationLedger) Reserve(
	podKey, nodeName string,
	selectDevices func(reserved map[string]struct{}) []string,
) (reservation, error) {
	if podKey == "" || nodeName == "" {
		return reservation{}, fmt.Errorf("pod key and node name are required")
	}
	if selectDevices == nil {
		return reservation{}, fmt.Errorf("device selector is required")
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if existing, found := l.byPod[podKey]; found {
		if existing.nodeName != nodeName {
			return reservation{}, fmt.Errorf("pod %q is already reserved on node %q", podKey, existing.nodeName)
		}
		return cloneReservation(existing), nil
	}

	reserved := make(map[string]struct{})
	for deviceID := range l.deviceOwner[nodeName] {
		reserved[deviceID] = struct{}{}
	}
	deviceIDs := append([]string(nil), selectDevices(reserved)...)
	if len(deviceIDs) == 0 {
		return reservation{}, ErrNoAvailableCombination
	}
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		if deviceID == "" {
			return reservation{}, fmt.Errorf("selector returned an empty device ID")
		}
		if _, duplicate := seen[deviceID]; duplicate {
			return reservation{}, fmt.Errorf("selector returned duplicate device ID %q", deviceID)
		}
		seen[deviceID] = struct{}{}
		if owner := l.deviceOwner[nodeName][deviceID]; owner != "" {
			return reservation{}, fmt.Errorf("device %q on node %q is reserved by %q", deviceID, nodeName, owner)
		}
	}

	if l.deviceOwner[nodeName] == nil {
		l.deviceOwner[nodeName] = make(map[string]string)
	}
	created := reservation{podKey: podKey, nodeName: nodeName, deviceIDs: deviceIDs}
	l.byPod[podKey] = created
	for _, deviceID := range deviceIDs {
		l.deviceOwner[nodeName][deviceID] = podKey
	}
	return cloneReservation(created), nil
}

func (l *reservationLedger) Release(podKey string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	reserved, found := l.byPod[podKey]
	if !found {
		return
	}
	delete(l.byPod, podKey)
	for _, deviceID := range reserved.deviceIDs {
		delete(l.deviceOwner[reserved.nodeName], deviceID)
	}
	if len(l.deviceOwner[reserved.nodeName]) == 0 {
		delete(l.deviceOwner, reserved.nodeName)
	}
}

func (l *reservationLedger) Get(podKey string) (reservation, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	reserved, found := l.byPod[podKey]
	return cloneReservation(reserved), found
}

func (l *reservationLedger) ReservedDeviceIDs(nodeName, excludePodKey string) map[string]struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	reserved := make(map[string]struct{})
	for deviceID, owner := range l.deviceOwner[nodeName] {
		if owner != excludePodKey {
			reserved[deviceID] = struct{}{}
		}
	}
	return reserved
}

func (l *reservationLedger) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.byPod)
}

func cloneReservation(value reservation) reservation {
	value.deviceIDs = append([]string(nil), value.deviceIDs...)
	return value
}
