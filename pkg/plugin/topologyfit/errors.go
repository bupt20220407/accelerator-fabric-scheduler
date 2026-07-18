package topologyfit

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
)

func newUnexpectedConfigError(config runtime.Object) error {
	return fmt.Errorf("%s does not accept plugin args in W1, got %T", Name, config)
}
