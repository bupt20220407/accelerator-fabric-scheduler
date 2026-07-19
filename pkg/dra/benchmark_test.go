package dra

import (
	"path/filepath"
	"testing"
)

func BenchmarkDRAClaimLifecycle(b *testing.B) {
	driver, err := NewDriver(DriverName, "accelerator-fabric-worker", filepath.Join(b.TempDir(), "state.json"), b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	claim := testClaim("benchmark", "benchmark-uid", "gpu0", "gpu1", "gpu2", "gpu3")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := driver.prepareClaim(claim); err != nil {
			b.Fatalf("prepare failed: %v", err)
		}
		if err := driver.unprepareClaim(claim.UID); err != nil {
			b.Fatalf("unprepare failed: %v", err)
		}
	}
}
