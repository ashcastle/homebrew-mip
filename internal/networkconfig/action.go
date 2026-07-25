package networkconfig

import (
	"context"
	"fmt"
)

type Action string

const (
	ActionDisplayDNS Action = "displaydns"
	ActionFlushDNS   Action = "flushdns"
	ActionRelease4   Action = "release"
	ActionRenew4     Action = "renew"
	ActionRelease6   Action = "release6"
	ActionRenew6     Action = "renew6"
)

type ActionRequest struct {
	Action   Action
	Adapters []Adapter
}

type ActionResult struct {
	Adapters        []string
	DNSCache        []DNSCacheEntry
	CacheRestricted bool
}

type DNSCacheEntry struct {
	Name    string
	Type    string
	Address string
}

type PrivilegeError struct {
	Action Action
}

func (e PrivilegeError) Error() string {
	return fmt.Sprintf("/%s requires administrator privileges on macOS", e.Action)
}

type Operator interface {
	Execute(context.Context, ActionRequest) (ActionResult, error)
}

func ExecuteAction(ctx context.Context, request ActionRequest) (ActionResult, error) {
	result, err := newPlatformOperator().Execute(ctx, request)
	if err != nil {
		return result, fmt.Errorf("execute /%s: %w", request.Action, err)
	}
	return result, nil
}
