//go:build darwin

package networkconfig

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
)

const (
	appleIPConfigPath = "/usr/sbin/ipconfig"
	dscacheutilPath   = "/usr/bin/dscacheutil"
	killallPath       = "/usr/bin/killall"
)

type darwinOperator struct {
	run     commandRunner
	geteuid func() int
}

func newPlatformOperator() Operator {
	return darwinOperator{
		run:     runCommand,
		geteuid: os.Geteuid,
	}
}

func (o darwinOperator) Execute(ctx context.Context, request ActionRequest) (ActionResult, error) {
	switch request.Action {
	case ActionDisplayDNS:
		return o.displayDNS(ctx)
	case ActionFlushDNS:
		return o.flushDNS(ctx)
	case ActionRelease4, ActionRenew4, ActionRelease6, ActionRenew6:
		return o.configureAdapters(ctx, request)
	default:
		return ActionResult{}, fmt.Errorf("unknown action %q", request.Action)
	}
}

func (o darwinOperator) displayDNS(ctx context.Context) (ActionResult, error) {
	if o.geteuid() != 0 {
		return ActionResult{}, PrivilegeError{Action: ActionDisplayDNS}
	}

	output, err := o.run(ctx, dscacheutilPath, []string{"-cachedump", "-entries", "Host"}, "")
	if err != nil {
		return ActionResult{}, fmt.Errorf("read DNS resolver cache: %w", err)
	}

	raw := strings.TrimSpace(string(output))
	return ActionResult{
		DNSCache:        parseDNSCacheEntries(raw),
		CacheRestricted: strings.Contains(strings.ToLower(raw), "unable to get details"),
	}, nil
}

func (o darwinOperator) flushDNS(ctx context.Context) (ActionResult, error) {
	if o.geteuid() != 0 {
		return ActionResult{}, PrivilegeError{Action: ActionFlushDNS}
	}
	if _, err := o.run(ctx, dscacheutilPath, []string{"-flushcache"}, ""); err != nil {
		return ActionResult{}, fmt.Errorf("flush DNS resolver cache: %w", err)
	}
	if _, err := o.run(ctx, killallPath, []string{"-HUP", "mDNSResponder"}, ""); err != nil {
		return ActionResult{}, fmt.Errorf("signal mDNSResponder after cache flush: %w", err)
	}
	return ActionResult{}, nil
}

func (o darwinOperator) configureAdapters(ctx context.Context, request ActionRequest) (ActionResult, error) {
	if o.geteuid() != 0 {
		return ActionResult{}, PrivilegeError{Action: request.Action}
	}
	if len(request.Adapters) == 0 {
		return ActionResult{}, fmt.Errorf("no eligible adapters")
	}

	mode, err := appleConfigurationMode(request.Action)
	if err != nil {
		return ActionResult{}, err
	}

	result := ActionResult{Adapters: make([]string, 0, len(request.Adapters))}
	for _, adapter := range request.Adapters {
		if adapter.InterfaceName == "" {
			return result, fmt.Errorf("adapter %q has no macOS interface name", adapter.Name)
		}
		if _, runErr := o.run(ctx, appleIPConfigPath, []string{"set", adapter.InterfaceName, mode}, ""); runErr != nil {
			return result, fmt.Errorf("%s (%s): %w", adapter.Name, adapter.InterfaceName, runErr)
		}
		result.Adapters = append(result.Adapters, adapter.Name)
	}
	return result, nil
}

func appleConfigurationMode(action Action) (string, error) {
	switch action {
	case ActionRelease4:
		return "NONE", nil
	case ActionRenew4:
		return "DHCP", nil
	case ActionRelease6:
		return "NONE-V6", nil
	case ActionRenew6:
		return "AUTOMATIC-V6", nil
	default:
		return "", fmt.Errorf("action %q does not configure an adapter", action)
	}
}

func parseDNSCacheEntries(output string) []DNSCacheEntry {
	var entries []DNSCacheEntry
	var name string
	seen := make(map[string]struct{})

	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			name = ""
			continue
		}

		key, value, ok := splitCacheLine(line)
		if !ok {
			continue
		}
		switch strings.ToLower(key) {
		case "name", "hostname", "canonical_name", "key":
			name = value
		case "ip_address", "ipv6_address", "address":
			ip := net.ParseIP(strings.SplitN(value, "%", 2)[0])
			if ip == nil {
				continue
			}
			recordType := "AAAA"
			if ip.To4() != nil {
				recordType = "A"
			}
			entryName := name
			if entryName == "" {
				entryName = "(name unavailable)"
			}
			deduplicationKey := strings.Join([]string{entryName, recordType, value}, "\x00")
			if _, exists := seen[deduplicationKey]; exists {
				continue
			}
			seen[deduplicationKey] = struct{}{}
			entries = append(entries, DNSCacheEntry{
				Name:    entryName,
				Type:    recordType,
				Address: value,
			})
		}
	}
	return entries
}

func splitCacheLine(line string) (string, string, bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}
