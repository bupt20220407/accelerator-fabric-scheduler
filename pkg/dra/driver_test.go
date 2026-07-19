package dra

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
)

func TestPrepareRestartConflictAndUnprepare(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(root, "state", "prepared.json")
	cdiDir := filepath.Join(root, "cdi")
	driver, err := NewDriver(DriverName, "accelerator-fabric-worker", statePath, cdiDir)
	if err != nil {
		t.Fatal(err)
	}
	claim := testClaim("claim-one", "uid-one", "gpu0", "gpu1", "gpu2", "gpu3")

	result, err := driver.PrepareResourceClaims(context.Background(), []*resourceapi.ResourceClaim{claim})
	if err != nil || result[claim.UID].Err != nil {
		t.Fatalf("PrepareResourceClaims() = (%+v, %v)", result, err)
	}
	devices := result[claim.UID].Devices
	if len(devices) != 4 || len(devices[0].CDIDeviceIDs) != 1 {
		t.Fatalf("prepared devices = %+v", devices)
	}
	cdiFiles, err := filepath.Glob(filepath.Join(cdiDir, "*.json"))
	if err != nil || len(cdiFiles) != 1 {
		t.Fatalf("CDI files = %v, err = %v", cdiFiles, err)
	}
	content, err := os.ReadFile(cdiFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "BUPT_DRA_DEVICE_IDS=accelerator-fabric-worker/gpu0") ||
		!strings.Contains(string(content), "accelerator-fabric-worker/gpu3") {
		t.Fatalf("CDI spec = %s", content)
	}

	restarted, err := NewDriver(DriverName, "accelerator-fabric-worker", statePath, cdiDir)
	if err != nil {
		t.Fatal(err)
	}
	result, err = restarted.PrepareResourceClaims(context.Background(), []*resourceapi.ResourceClaim{claim})
	if err != nil || result[claim.UID].Err != nil || len(result[claim.UID].Devices) != 4 {
		t.Fatalf("idempotent PrepareResourceClaims() = (%+v, %v)", result, err)
	}
	conflict := testClaim("claim-two", "uid-two", "gpu0")
	result, err = restarted.PrepareResourceClaims(context.Background(), []*resourceapi.ResourceClaim{conflict})
	if err != nil || result[conflict.UID].Err == nil {
		t.Fatalf("conflicting PrepareResourceClaims() = (%+v, %v)", result, err)
	}

	claimRef := kubeletplugin.NamespacedObject{
		NamespacedName: types.NamespacedName{Namespace: claim.Namespace, Name: claim.Name},
		UID:            claim.UID,
	}
	unprepared, err := restarted.UnprepareResourceClaims(context.Background(), []kubeletplugin.NamespacedObject{claimRef})
	if err != nil || unprepared[claim.UID] != nil {
		t.Fatalf("UnprepareResourceClaims() = (%+v, %v)", unprepared, err)
	}
	if _, err := os.Stat(cdiFiles[0]); !os.IsNotExist(err) {
		t.Fatalf("CDI file remained after Unprepare: %v", err)
	}
	unprepared, err = restarted.UnprepareResourceClaims(context.Background(), []kubeletplugin.NamespacedObject{claimRef})
	if err != nil || unprepared[claim.UID] != nil {
		t.Fatalf("idempotent UnprepareResourceClaims() = (%+v, %v)", unprepared, err)
	}
	clean, err := NewDriver(DriverName, "accelerator-fabric-worker", statePath, cdiDir)
	if err != nil || len(clean.prepared) != 0 || len(clean.owners) != 0 {
		t.Fatalf("reloaded clean state = (%+v, %v)", clean, err)
	}
}

func TestPrepareRejectsWrongPoolAndUnallocatedClaim(t *testing.T) {
	driver, err := NewDriver(DriverName, "accelerator-fabric-worker", filepath.Join(t.TempDir(), "state.json"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	unallocated := &resourceapi.ResourceClaim{ObjectMeta: metav1.ObjectMeta{Name: "empty", Namespace: "default", UID: "empty-uid"}}
	result, _ := driver.PrepareResourceClaims(context.Background(), []*resourceapi.ResourceClaim{unallocated})
	if result[unallocated.UID].Err == nil {
		t.Fatal("PrepareResourceClaims() accepted an unallocated claim")
	}
	wrongPool := testClaim("wrong", "wrong-uid", "gpu0")
	wrongPool.Status.Allocation.Devices.Results[0].Pool = "another-node"
	result, _ = driver.PrepareResourceClaims(context.Background(), []*resourceapi.ResourceClaim{wrongPool})
	if result[wrongPool.UID].Err == nil {
		t.Fatal("PrepareResourceClaims() accepted a device from another node pool")
	}
}

func testClaim(name string, uid types.UID, deviceIDs ...string) *resourceapi.ResourceClaim {
	results := make([]resourceapi.DeviceRequestAllocationResult, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		results = append(results, resourceapi.DeviceRequestAllocationResult{
			Request: "accelerators",
			Driver:  DriverName,
			Pool:    "accelerator-fabric-worker",
			Device:  deviceID,
		})
	}
	return &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", UID: uid},
		Status: resourceapi.ResourceClaimStatus{Allocation: &resourceapi.AllocationResult{
			Devices: resourceapi.DeviceAllocationResult{Results: results},
		}},
	}
}
