//go:build !darwin

package networkconfig

import (
	"context"
	"fmt"
	"runtime"
)

type unsupportedOperator struct{}

func newPlatformOperator() Operator {
	return unsupportedOperator{}
}

func (unsupportedOperator) Execute(context.Context, ActionRequest) (ActionResult, error) {
	return ActionResult{}, fmt.Errorf("unsupported operating system %q; network actions currently require macOS", runtime.GOOS)
}
