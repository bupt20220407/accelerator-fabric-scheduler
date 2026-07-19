package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestAddToScheme(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	kinds, _, err := scheme.ObjectKinds(&AcceleratorTopology{})
	if err != nil {
		t.Fatalf("ObjectKinds() error = %v", err)
	}
	if len(kinds) != 1 || kinds[0].Group != GroupName || kinds[0].Version != "v1alpha1" {
		t.Fatalf("unexpected topology GVKs: %v", kinds)
	}
}
