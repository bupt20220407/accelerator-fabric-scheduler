package dra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/klog/v2"

	"github.com/bupt20220407/accelerator-fabric-scheduler/pkg/observability"
)

const stateVersion = 1

type allocatedDevice struct {
	Request string `json:"request"`
	Pool    string `json:"pool"`
	Device  string `json:"device"`
	ShareID string `json:"shareID,omitempty"`
}

type preparedClaim struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	UID       types.UID         `json:"uid"`
	Devices   []allocatedDevice `json:"devices"`
	CDIFile   string            `json:"cdiFile"`
	CDIID     string            `json:"cdiID"`
}

type persistedState struct {
	Version int                      `json:"version"`
	Claims  map[string]preparedClaim `json:"claims"`
}

type Driver struct {
	mu         sync.Mutex
	driverName string
	nodeName   string
	statePath  string
	cdiDir     string
	prepared   map[types.UID]preparedClaim
	owners     map[string]types.UID
}

func NewDriver(driverName, nodeName, statePath, cdiDir string) (*Driver, error) {
	if driverName == "" || nodeName == "" || statePath == "" || cdiDir == "" {
		return nil, fmt.Errorf("driver name, node name, state path, and CDI directory are required")
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o750); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	if err := os.MkdirAll(cdiDir, 0o755); err != nil {
		return nil, fmt.Errorf("create CDI directory: %w", err)
	}
	driver := &Driver{
		driverName: driverName,
		nodeName:   nodeName,
		statePath:  statePath,
		cdiDir:     cdiDir,
		prepared:   make(map[types.UID]preparedClaim),
		owners:     make(map[string]types.UID),
	}
	if err := driver.load(); err != nil {
		return nil, err
	}
	return driver, nil
}

func (d *Driver) PrepareResourceClaims(
	ctx context.Context,
	claims []*resourceapi.ResourceClaim,
) (map[types.UID]kubeletplugin.PrepareResult, error) {
	results := make(map[types.UID]kubeletplugin.PrepareResult, len(claims))
	for _, claim := range claims {
		started := time.Now()
		devices, err := d.prepareClaim(claim)
		results[claim.UID] = kubeletplugin.PrepareResult{Devices: devices, Err: err}
		result := observability.ResultSuccess
		if err != nil {
			result = observability.ResultError
		}
		observability.ObserveDRAOperation("prepare", result, time.Since(started))
		if err == nil {
			klog.FromContext(ctx).Info("prepared authoritative DRA allocation", "claim", klog.KObj(claim), "claimUID", claim.UID, "devices", devices)
		}
	}
	observability.SetDRAPreparedClaims(d.PreparedCount())
	return results, nil
}

func (d *Driver) UnprepareResourceClaims(
	ctx context.Context,
	claims []kubeletplugin.NamespacedObject,
) (map[types.UID]error, error) {
	results := make(map[types.UID]error, len(claims))
	for _, claim := range claims {
		started := time.Now()
		err := d.unprepareClaim(claim.UID)
		results[claim.UID] = err
		result := observability.ResultSuccess
		if err != nil {
			result = observability.ResultError
		}
		observability.ObserveDRAOperation("unprepare", result, time.Since(started))
		if err == nil {
			klog.FromContext(ctx).Info("unprepared authoritative DRA allocation", "claim", claim.Namespace+"/"+claim.Name, "claimUID", claim.UID)
		}
	}
	observability.SetDRAPreparedClaims(d.PreparedCount())
	return results, nil
}

func (d *Driver) HandleError(ctx context.Context, err error, msg string) {
	utilruntime.HandleErrorWithContext(ctx, err, msg)
}

func (d *Driver) PreparedCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.prepared)
}

func (d *Driver) prepareClaim(claim *resourceapi.ResourceClaim) ([]kubeletplugin.Device, error) {
	if claim == nil || claim.UID == "" {
		return nil, fmt.Errorf("claim and claim UID are required")
	}
	if claim.Status.Allocation == nil {
		return nil, fmt.Errorf("claim %s/%s is not allocated", claim.Namespace, claim.Name)
	}
	allocated := make([]allocatedDevice, 0)
	for _, result := range claim.Status.Allocation.Devices.Results {
		if result.Driver != d.driverName {
			continue
		}
		if result.Pool != d.nodeName {
			return nil, fmt.Errorf("device %q belongs to pool %q, not node pool %q", result.Device, result.Pool, d.nodeName)
		}
		shareID := ""
		if result.ShareID != nil {
			shareID = string(*result.ShareID)
		}
		allocated = append(allocated, allocatedDevice{
			Request: result.Request,
			Pool:    result.Pool,
			Device:  result.Device,
			ShareID: shareID,
		})
	}
	if len(allocated) == 0 {
		return nil, fmt.Errorf("claim %s/%s has no allocation for driver %q", claim.Namespace, claim.Name, d.driverName)
	}
	sort.Slice(allocated, func(left, right int) bool {
		if allocated[left].Pool != allocated[right].Pool {
			return allocated[left].Pool < allocated[right].Pool
		}
		return allocated[left].Device < allocated[right].Device
	})

	d.mu.Lock()
	defer d.mu.Unlock()
	if existing, found := d.prepared[claim.UID]; found {
		if !devicesEqual(existing.Devices, allocated) {
			return nil, fmt.Errorf("claim %q allocation changed after preparation", claim.UID)
		}
		if err := d.writeCDI(existing); err != nil {
			return nil, err
		}
		return d.kubeletDevices(existing), nil
	}
	for _, device := range allocated {
		key := deviceKey(device)
		if owner := d.owners[key]; owner != "" && owner != claim.UID {
			return nil, fmt.Errorf("allocated device %q is already prepared for claim %q", key, owner)
		}
	}

	prepared := preparedClaim{
		Namespace: claim.Namespace,
		Name:      claim.Name,
		UID:       claim.UID,
		Devices:   allocated,
		CDIFile:   filepath.Join(d.cdiDir, "accelerator-"+string(claim.UID)+".json"),
		CDIID:     d.driverName + "/device=claim-" + string(claim.UID),
	}
	if err := d.writeCDI(prepared); err != nil {
		return nil, err
	}
	d.prepared[claim.UID] = prepared
	for _, device := range allocated {
		d.owners[deviceKey(device)] = claim.UID
	}
	if err := d.persist(); err != nil {
		delete(d.prepared, claim.UID)
		for _, device := range allocated {
			delete(d.owners, deviceKey(device))
		}
		_ = os.Remove(prepared.CDIFile)
		return nil, err
	}
	return d.kubeletDevices(prepared), nil
}

func (d *Driver) unprepareClaim(uid types.UID) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	prepared, found := d.prepared[uid]
	if !found {
		return nil
	}
	if err := os.Remove(prepared.CDIFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove CDI file: %w", err)
	}
	delete(d.prepared, uid)
	for _, device := range prepared.Devices {
		delete(d.owners, deviceKey(device))
	}
	if err := d.persist(); err != nil {
		d.prepared[uid] = prepared
		for _, device := range prepared.Devices {
			d.owners[deviceKey(device)] = uid
		}
		if restoreErr := d.writeCDI(prepared); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore CDI file after state failure: %w", restoreErr))
		}
		return err
	}
	return nil
}

func (d *Driver) load() error {
	content, err := os.ReadFile(d.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read DRA state: %w", err)
	}
	var state persistedState
	if err := json.Unmarshal(content, &state); err != nil {
		return fmt.Errorf("decode DRA state: %w", err)
	}
	if state.Version != stateVersion {
		return fmt.Errorf("unsupported DRA state version %d", state.Version)
	}
	for key, prepared := range state.Claims {
		if string(prepared.UID) != key || prepared.UID == "" {
			return fmt.Errorf("invalid persisted claim key %q", key)
		}
		for _, device := range prepared.Devices {
			deviceID := deviceKey(device)
			if owner := d.owners[deviceID]; owner != "" {
				return fmt.Errorf("persisted device %q is owned by claims %q and %q", deviceID, owner, prepared.UID)
			}
			d.owners[deviceID] = prepared.UID
		}
		d.prepared[prepared.UID] = prepared
		if err := d.writeCDI(prepared); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) persist() error {
	state := persistedState{Version: stateVersion, Claims: make(map[string]preparedClaim, len(d.prepared))}
	for uid, prepared := range d.prepared {
		state.Claims[string(uid)] = prepared
	}
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode DRA state: %w", err)
	}
	if err := atomicWrite(d.statePath, content, 0o600); err != nil {
		return fmt.Errorf("persist DRA state: %w", err)
	}
	return nil
}

func (d *Driver) writeCDI(prepared preparedClaim) error {
	deviceIDs := make([]string, 0, len(prepared.Devices))
	for _, device := range prepared.Devices {
		deviceIDs = append(deviceIDs, deviceKey(device))
	}
	spec := cdiSpec{
		CDIVersion: "0.3.0",
		Kind:       d.driverName + "/device",
		Devices: []cdiDevice{{
			Name: "claim-" + string(prepared.UID),
			ContainerEdits: cdiContainerEdits{Env: []string{
				"BUPT_DRA_CLAIM_UID=" + string(prepared.UID),
				"BUPT_DRA_DEVICE_IDS=" + strings.Join(deviceIDs, ","),
			}},
		}},
	}
	content, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("encode CDI spec: %w", err)
	}
	if err := atomicWrite(prepared.CDIFile, content, 0o644); err != nil {
		return fmt.Errorf("write CDI spec: %w", err)
	}
	return nil
}

func (d *Driver) kubeletDevices(prepared preparedClaim) []kubeletplugin.Device {
	devices := make([]kubeletplugin.Device, 0, len(prepared.Devices))
	for index, allocated := range prepared.Devices {
		device := kubeletplugin.Device{
			Requests:   []string{allocated.Request},
			PoolName:   allocated.Pool,
			DeviceName: allocated.Device,
		}
		if allocated.ShareID != "" {
			shareID := types.UID(allocated.ShareID)
			device.ShareID = &shareID
		}
		if index == 0 {
			device.CDIDeviceIDs = []string{prepared.CDIID}
		}
		devices = append(devices, device)
	}
	return devices
}

func deviceKey(device allocatedDevice) string {
	return device.Pool + "/" + device.Device
}

func devicesEqual(left, right []allocatedDevice) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func atomicWrite(path string, content []byte, mode os.FileMode) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, content, mode); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

type cdiSpec struct {
	CDIVersion string      `json:"cdiVersion"`
	Kind       string      `json:"kind"`
	Devices    []cdiDevice `json:"devices"`
}

type cdiDevice struct {
	Name           string            `json:"name"`
	ContainerEdits cdiContainerEdits `json:"containerEdits"`
}

type cdiContainerEdits struct {
	Env []string `json:"env"`
}
