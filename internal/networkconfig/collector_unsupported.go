//go:build !darwin

package networkconfig

import (
	"context"
	"fmt"
	"runtime"
)

type unsupportedCollector struct{}

func newPlatformCollector() Collector {
	return unsupportedCollector{}
}

func (unsupportedCollector) Collect(context.Context) (Configuration, error) {
	return Configuration{}, fmt.Errorf("unsupported operating system %q; ipconfig currently requires macOS", runtime.GOOS)
}
