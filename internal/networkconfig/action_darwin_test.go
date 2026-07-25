//go:build darwin

package networkconfig

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDarwinOperatorRequiresRootBeforeRunningCommands(t *testing.T) {
	t.Parallel()

	for _, action := range []Action{
		ActionDisplayDNS,
		ActionFlushDNS,
		ActionRelease4,
		ActionRenew4,
		ActionRelease6,
		ActionRenew6,
	} {
		action := action
		t.Run(string(action), func(t *testing.T) {
			t.Parallel()
			ran := false
			operator := darwinOperator{
				geteuid: func() int { return 501 },
				run: func(context.Context, string, []string, string) ([]byte, error) {
					ran = true
					return nil, nil
				},
			}

			request := ActionRequest{Action: action}
			if action != ActionDisplayDNS && action != ActionFlushDNS {
				request.Adapters = []Adapter{{Name: "Wi-Fi", InterfaceName: "en0"}}
			}
			_, err := operator.Execute(context.Background(), request)

			var privilegeError PrivilegeError
			if !errors.As(err, &privilegeError) {
				t.Fatalf("expected PrivilegeError, got %v", err)
			}
			if ran {
				t.Fatal("command runner must not be called without root")
			}
		})
	}
}

func TestDarwinOperatorMapsAdapterActionsToAppleIPConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		action Action
		mode   string
	}{
		{action: ActionRelease4, mode: "NONE"},
		{action: ActionRenew4, mode: "DHCP"},
		{action: ActionRelease6, mode: "NONE-V6"},
		{action: ActionRenew6, mode: "AUTOMATIC-V6"},
	}

	for _, test := range tests {
		test := test
		t.Run(string(test.action), func(t *testing.T) {
			t.Parallel()
			var path string
			var args []string
			operator := darwinOperator{
				geteuid: func() int { return 0 },
				run: func(_ context.Context, commandPath string, commandArgs []string, _ string) ([]byte, error) {
					path = commandPath
					args = append([]string(nil), commandArgs...)
					return nil, nil
				},
			}

			result, err := operator.Execute(context.Background(), ActionRequest{
				Action: test.action,
				Adapters: []Adapter{
					{Name: "Wi-Fi", InterfaceName: "en0"},
				},
			})
			if err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			if path != appleIPConfigPath {
				t.Fatalf("path=%q, want %q", path, appleIPConfigPath)
			}
			wantArgs := []string{"set", "en0", test.mode}
			if !reflect.DeepEqual(args, wantArgs) {
				t.Fatalf("args=%#v, want %#v", args, wantArgs)
			}
			if !reflect.DeepEqual(result.Adapters, []string{"Wi-Fi"}) {
				t.Fatalf("unexpected result adapters: %#v", result.Adapters)
			}
		})
	}
}

func TestDarwinOperatorStopsOnFirstAdapterFailure(t *testing.T) {
	t.Parallel()

	var calls [][]string
	operator := darwinOperator{
		geteuid: func() int { return 0 },
		run: func(_ context.Context, _ string, args []string, _ string) ([]byte, error) {
			calls = append(calls, append([]string(nil), args...))
			if args[1] == "en5" {
				return nil, errors.New("interface unavailable")
			}
			return nil, nil
		},
	}

	result, err := operator.Execute(context.Background(), ActionRequest{
		Action: ActionRenew4,
		Adapters: []Adapter{
			{Name: "Wi-Fi", InterfaceName: "en0"},
			{Name: "USB Ethernet", InterfaceName: "en5"},
			{Name: "Other", InterfaceName: "en6"},
		},
	})

	if err == nil || !strings.Contains(err.Error(), "USB Ethernet (en5)") {
		t.Fatalf("expected contextual adapter failure, got %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected execution to stop after two calls, got %#v", calls)
	}
	if !reflect.DeepEqual(result.Adapters, []string{"Wi-Fi"}) {
		t.Fatalf("expected completed adapter to be preserved, got %#v", result.Adapters)
	}
}

func TestDarwinOperatorDisplaysDNSCache(t *testing.T) {
	t.Parallel()

	operator := darwinOperator{
		geteuid: func() int { return 0 },
		run: func(_ context.Context, path string, args []string, _ string) ([]byte, error) {
			if path != dscacheutilPath || !reflect.DeepEqual(args, []string{"-cachedump", "-entries", "Host"}) {
				t.Fatalf("unexpected command: %s %#v", path, args)
			}
			return []byte(`
name: example.test
ip_address: 192.0.2.10
ipv6_address: 2001:db8::10

hostname: duplicate.test
ip_address: 192.0.2.20
ip_address: 192.0.2.20
`), nil
		},
	}

	result, err := operator.Execute(context.Background(), ActionRequest{Action: ActionDisplayDNS})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(result.DNSCache) != 3 {
		t.Fatalf("expected three unique cache records, got %#v", result.DNSCache)
	}
	if result.DNSCache[0].Name != "example.test" || result.DNSCache[0].Type != "A" {
		t.Fatalf("unexpected first cache record: %#v", result.DNSCache[0])
	}
	if result.DNSCache[1].Type != "AAAA" {
		t.Fatalf("unexpected IPv6 cache record: %#v", result.DNSCache[1])
	}
}

func TestDarwinOperatorReportsRestrictedDNSCache(t *testing.T) {
	t.Parallel()

	operator := darwinOperator{
		geteuid: func() int { return 0 },
		run: func(context.Context, string, []string, string) ([]byte, error) {
			return []byte("Unable to get details from the cache node\n"), nil
		},
	}

	result, err := operator.Execute(context.Background(), ActionRequest{Action: ActionDisplayDNS})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.CacheRestricted || len(result.DNSCache) != 0 {
		t.Fatalf("unexpected restricted result: %#v", result)
	}
}

func TestDarwinOperatorFlushesDirectoryServiceCache(t *testing.T) {
	t.Parallel()

	type commandCall struct {
		path string
		args []string
	}
	var calls []commandCall
	operator := darwinOperator{
		geteuid: func() int { return 0 },
		run: func(_ context.Context, commandPath string, commandArgs []string, _ string) ([]byte, error) {
			calls = append(calls, commandCall{
				path: commandPath,
				args: append([]string(nil), commandArgs...),
			})
			return nil, nil
		},
	}

	if _, err := operator.Execute(context.Background(), ActionRequest{Action: ActionFlushDNS}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	want := []commandCall{
		{path: dscacheutilPath, args: []string{"-flushcache"}},
		{path: killallPath, args: []string{"-HUP", "mDNSResponder"}},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected flush commands: %#v, want %#v", calls, want)
	}
}

func TestDarwinOperatorReportsMDNSResponderSignalFailure(t *testing.T) {
	t.Parallel()

	operator := darwinOperator{
		geteuid: func() int { return 0 },
		run: func(_ context.Context, path string, _ []string, _ string) ([]byte, error) {
			if path == killallPath {
				return nil, errors.New("process unavailable")
			}
			return nil, nil
		},
	}

	_, err := operator.Execute(context.Background(), ActionRequest{Action: ActionFlushDNS})
	if err == nil || !strings.Contains(err.Error(), "signal mDNSResponder") {
		t.Fatalf("expected mDNSResponder error, got %v", err)
	}
}

func TestParseDNSCacheEntriesIgnoresMalformedData(t *testing.T) {
	t.Parallel()

	entries := parseDNSCacheEntries(`
name:
address: not-an-ip
address: 203.0.113.5
unrelated line
`)
	if len(entries) != 1 || entries[0].Name != "(name unavailable)" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}
