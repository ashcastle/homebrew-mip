package ipconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ashcastle/homebrew-mip/internal/networkconfig"
)

func TestRunAcceptsWindowsAllSyntax(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"/ALL"}, displayDependencies())

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}

	for _, expected := range []string{
		"Windows IP Configuration",
		"Host Name",
		"Wireless LAN adapter Wi-Fi:",
		"DHCP Enabled",
		"192.168.0.20(Preferred)",
		"DHCPv6 IAID",
		"123456789",
		"DHCPv6 Client DUID",
		"00:01:00:01:AA:BB:CC:DD",
		"NetBIOS over Tcpip",
		"Not applicable on macOS",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("expected output to contain %q, got:\n%s", expected, stdout.String())
		}
	}
}

func TestRunJSONProducesValidArray(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"-json"}, displayDependencies())

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}

	var adapters []networkconfig.Adapter
	if err := json.Unmarshal(stdout.Bytes(), &adapters); err != nil {
		t.Fatalf("expected valid JSON array, got error %v and payload %q", err, stdout.String())
	}
	if len(adapters) != 2 || adapters[0].InterfaceName != "en0" {
		t.Fatalf("unexpected JSON adapters: %#v", adapters)
	}
	if strings.Contains(stdout.String(), "Windows IP Configuration") {
		t.Fatalf("expected JSON without text banner, got %q", stdout.String())
	}
}

func TestRunFiltersAdaptersWithCaseInsensitiveWildcard(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"-interface", "wi*"}, displayDependencies())

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Wi-Fi") {
		t.Fatalf("expected Wi-Fi adapter, got %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "USB Ethernet") {
		t.Fatalf("did not expect USB Ethernet adapter, got %q", stdout.String())
	}
}

func TestRunRenewMapsSelectedAdapter(t *testing.T) {
	t.Parallel()

	var captured networkconfig.ActionRequest
	deps := displayDependencies()
	deps.execute = func(_ context.Context, request networkconfig.ActionRequest) (networkconfig.ActionResult, error) {
		captured = request
		return networkconfig.ActionResult{Adapters: []string{request.Adapters[0].Name}}, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"/ReNeW", "wi*"}, deps)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if captured.Action != networkconfig.ActionRenew4 {
		t.Fatalf("action=%q, want renew", captured.Action)
	}
	if len(captured.Adapters) != 1 || captured.Adapters[0].InterfaceName != "en0" {
		t.Fatalf("unexpected selected adapters: %#v", captured.Adapters)
	}
	if !strings.Contains(stdout.String(), "Successfully renewed the IPv4 configuration") {
		t.Fatalf("unexpected action output: %q", stdout.String())
	}
}

func TestRunRenewWithoutPatternSelectsAllDHCPAdapters(t *testing.T) {
	t.Parallel()

	var captured networkconfig.ActionRequest
	deps := displayDependencies()
	deps.execute = func(_ context.Context, request networkconfig.ActionRequest) (networkconfig.ActionResult, error) {
		captured = request
		return networkconfig.ActionResult{Adapters: []string{"Wi-Fi", "USB Ethernet"}}, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"/renew"}, deps)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if len(captured.Adapters) != 2 {
		t.Fatalf("expected two DHCP adapters, got %#v", captured.Adapters)
	}
}

func TestRunRenew6ExcludesTunnelAdapters(t *testing.T) {
	t.Parallel()

	config := testConfiguration()
	config.Adapters = append(config.Adapters, networkconfig.Adapter{
		Name:          "Tailscale",
		InterfaceName: "utun4",
		Kind:          networkconfig.AdapterTunnel,
		IPv6Automatic: true,
	})

	var captured networkconfig.ActionRequest
	deps := displayDependencies()
	deps.collect = func(context.Context) (networkconfig.Configuration, error) { return config, nil }
	deps.execute = func(_ context.Context, request networkconfig.ActionRequest) (networkconfig.ActionResult, error) {
		captured = request
		return networkconfig.ActionResult{Adapters: []string{"Wi-Fi"}}, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"/renew6"}, deps)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if len(captured.Adapters) != 1 || captured.Adapters[0].InterfaceName != "en0" {
		t.Fatalf("unexpected IPv6 targets: %#v", captured.Adapters)
	}
}

func TestRunRejectsIneligibleAdapterBeforeExecution(t *testing.T) {
	t.Parallel()

	executed := false
	deps := displayDependencies()
	deps.execute = func(context.Context, networkconfig.ActionRequest) (networkconfig.ActionResult, error) {
		executed = true
		return networkconfig.ActionResult{}, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"/renew6", "USB*"}, deps)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if executed {
		t.Fatal("executor must not run for an ineligible adapter")
	}
	if !strings.Contains(stderr.String(), "not eligible") {
		t.Fatalf("expected eligibility error, got %q", stderr.String())
	}
}

func TestRunDisplayDNSDoesNotCollectInterfaces(t *testing.T) {
	t.Parallel()

	collected := false
	deps := displayDependencies()
	deps.collect = func(context.Context) (networkconfig.Configuration, error) {
		collected = true
		return networkconfig.Configuration{}, nil
	}
	deps.execute = func(_ context.Context, request networkconfig.ActionRequest) (networkconfig.ActionResult, error) {
		if request.Action != networkconfig.ActionDisplayDNS {
			t.Fatalf("action=%q, want displaydns", request.Action)
		}
		return networkconfig.ActionResult{DNSCache: []networkconfig.DNSCacheEntry{
			{Name: "example.test", Type: "A", Address: "192.0.2.10"},
		}}, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"/displaydns"}, deps)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if collected {
		t.Fatal("displaydns must not collect adapters")
	}
	for _, expected := range []string{"DNS Resolver Cache", "example.test", "192.0.2.10", "Not available on macOS"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("expected %q in output: %s", expected, stdout.String())
		}
	}
}

func TestRunFlushDNS(t *testing.T) {
	t.Parallel()

	deps := displayDependencies()
	deps.execute = func(_ context.Context, request networkconfig.ActionRequest) (networkconfig.ActionResult, error) {
		if request.Action != networkconfig.ActionFlushDNS {
			t.Fatalf("action=%q, want flushdns", request.Action)
		}
		return networkconfig.ActionResult{}, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"/flushdns"}, deps)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Successfully flushed the DNS Resolver Cache") {
		t.Fatalf("unexpected flush output: %q", stdout.String())
	}
}

func TestRunPrivilegeErrorUsesAbsoluteExecutable(t *testing.T) {
	t.Parallel()

	deps := displayDependencies()
	deps.execute = func(_ context.Context, request networkconfig.ActionRequest) (networkconfig.ActionResult, error) {
		return networkconfig.ActionResult{}, fmt.Errorf("wrapped: %w", networkconfig.PrivilegeError{Action: request.Action})
	}
	deps.executable = func() (string, error) {
		return "/Applications/Homebrew Tools/ipconfig", nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"/renew", "Wi-Fi"}, deps)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "requires administrator privileges") {
		t.Fatalf("expected privilege error, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), `sudo '/Applications/Homebrew Tools/ipconfig' /renew Wi-Fi`) {
		t.Fatalf("expected absolute retry command, got %q", stderr.String())
	}
}

func TestRunPrivilegeErrorPreservesPartialCompletion(t *testing.T) {
	t.Parallel()

	deps := displayDependencies()
	deps.execute = func(_ context.Context, request networkconfig.ActionRequest) (networkconfig.ActionResult, error) {
		return networkconfig.ActionResult{Adapters: []string{"Wi-Fi"}},
			networkconfig.PrivilegeError{Action: request.Action}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"/renew"}, deps)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "Completed before failure: Wi-Fi") {
		t.Fatalf("expected partial completion details, got %q", stderr.String())
	}
}

func TestRunReportsPartialActionCompletion(t *testing.T) {
	t.Parallel()

	deps := displayDependencies()
	deps.execute = func(context.Context, networkconfig.ActionRequest) (networkconfig.ActionResult, error) {
		return networkconfig.ActionResult{Adapters: []string{"Wi-Fi"}}, errors.New("USB Ethernet failed")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"/renew"}, deps)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "Completed before failure: Wi-Fi") {
		t.Fatalf("expected partial completion details, got %q", stderr.String())
	}
}

func TestRunNotApplicableOperations(t *testing.T) {
	t.Parallel()

	for _, option := range []string{"/registerdns", "/showclassid", "/setclassid", "/allcompartments"} {
		option := option
		t.Run(option, func(t *testing.T) {
			t.Parallel()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := run(&stdout, &stderr, []string{option}, displayDependencies())

			if exitCode != 1 {
				t.Fatalf("expected exit code 1, got %d", exitCode)
			}
			if !strings.Contains(stdout.String(), "Not applicable on macOS") {
				t.Fatalf("unexpected output: %q", stdout.String())
			}
		})
	}
}

func TestRunHelpVariantsDoNotCollect(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"help"}, {"/?"}, {"-h"}, {"--help"}} {
		args := args
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			t.Parallel()
			collected := false
			deps := displayDependencies()
			deps.collect = func(context.Context) (networkconfig.Configuration, error) {
				collected = true
				return networkconfig.Configuration{}, nil
			}

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := run(&stdout, &stderr, args, deps)

			if exitCode != 0 {
				t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
			}
			if collected {
				t.Fatal("collector must not run for help")
			}
			if !strings.Contains(stdout.String(), "WINDOWS-COMPATIBLE OPTIONS") {
				t.Fatalf("expected help output, got %q", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no duplicate help on stderr, got %q", stderr.String())
			}
		})
	}
}

func TestRunRejectsInvalidActionArguments(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"/flushdns", "en0"},
		{"/renew", "-json"},
		{"/renew", "en0", "/all"},
		{"/unknown"},
	}
	for _, args := range tests {
		args := args
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			t.Parallel()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if exitCode := run(&stdout, &stderr, args, displayDependencies()); exitCode != 2 {
				t.Fatalf("expected exit code 2, got %d, stderr=%q", exitCode, stderr.String())
			}
		})
	}
}

func TestRunPropagatesWriterFailure(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	exitCode := run(failingWriter{}, &stderr, nil, displayDependencies())

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "write output") {
		t.Fatalf("expected write error, got %q", stderr.String())
	}
}

func TestRunReportsCollectorFailure(t *testing.T) {
	t.Parallel()

	deps := displayDependencies()
	deps.collect = func(context.Context) (networkconfig.Configuration, error) {
		return networkconfig.Configuration{}, errors.New("system configuration unavailable")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, nil, deps)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "system configuration unavailable") {
		t.Fatalf("expected collector error, got %q", stderr.String())
	}
}

func TestWildcardMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{pattern: "Local*", value: "Local Area Connection", want: true},
		{pattern: "*Con*", value: "Local Area Connection", want: true},
		{pattern: "wi*", value: "Wi-Fi", want: true},
		{pattern: "en?", value: "en0", want: false},
		{pattern: "Ethernet", value: "Wi-Fi", want: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.pattern+"/"+test.value, func(t *testing.T) {
			t.Parallel()
			if got := wildcardMatch(test.pattern, test.value); got != test.want {
				t.Fatalf("wildcardMatch(%q, %q)=%t, want %t", test.pattern, test.value, got, test.want)
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	t.Parallel()

	if got := shellQuote("/opt/homebrew/bin/ipconfig"); got != "/opt/homebrew/bin/ipconfig" {
		t.Fatalf("unexpected safe shell value: %q", got)
	}
	if got := shellQuote("Wi-Fi Adapter"); got != "'Wi-Fi Adapter'" {
		t.Fatalf("unexpected quoted shell value: %q", got)
	}
	if got := shellQuote("user's adapter"); got != `'user'"'"'s adapter'` {
		t.Fatalf("unexpected apostrophe quoting: %q", got)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("disk full")
}

func displayDependencies() dependencies {
	return dependencies{
		collect: func(context.Context) (networkconfig.Configuration, error) {
			return testConfiguration(), nil
		},
		execute: func(context.Context, networkconfig.ActionRequest) (networkconfig.ActionResult, error) {
			return networkconfig.ActionResult{}, errors.New("unexpected executor call")
		},
		executable: func() (string, error) {
			return "/opt/homebrew/bin/ipconfig", nil
		},
	}
}

func testConfiguration() networkconfig.Configuration {
	return networkconfig.Configuration{
		Host: networkconfig.HostInfo{
			HostName:         "macbook",
			PrimaryDNSSuffix: "example.test",
			NodeType:         "Hybrid",
			DHCPv6DUID:       "00:01:00:01:AA:BB:CC:DD",
		},
		Adapters: []networkconfig.Adapter{
			{
				Name:                     "Wi-Fi",
				Description:              "AirPort (en0)",
				InterfaceName:            "en0",
				Kind:                     networkconfig.AdapterWireless,
				Connected:                true,
				PhysicalAddress:          "AA-BB-CC-DD-EE-FF",
				DHCPEnabled:              true,
				AutoconfigurationEnabled: true,
				IPv6Automatic:            true,
				IPv4Addresses: []networkconfig.IPv4Assignment{
					{Address: "192.168.0.20", SubnetMask: "255.255.255.0"},
				},
				DefaultGateways: []string{"192.168.0.1"},
				DHCPServer:      "192.168.0.1",
				DNSServers:      []string{"210.220.163.82", "219.250.36.130"},
				LeaseObtained:   "07/25/2026 16:57:17",
				LeaseExpires:    "07/25/2026 18:57:17",
				DHCPv6IAID:      "123456789",
			},
			{
				Name:            "USB Ethernet",
				Description:     "Ethernet (en5)",
				InterfaceName:   "en5",
				Kind:            networkconfig.AdapterEthernet,
				PhysicalAddress: "11-22-33-44-55-66",
				DHCPEnabled:     true,
			},
		},
	}
}
