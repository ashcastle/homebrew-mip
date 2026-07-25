package ipconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ashcastle/homebrew-mip/internal/networkconfig"
)

func TestRunAcceptsWindowsAllSyntax(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"/ALL"}, dependencies{
		collect: func(context.Context) (networkconfig.Configuration, error) {
			return testConfiguration(), nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}

	for _, expected := range []string{
		"Windows IP Configuration",
		"Host Name",
		"Wireless LAN adapter Wi-Fi:",
		"Description",
		"DHCP Enabled",
		"192.168.0.20(Preferred)",
		"192.168.0.1",
		"210.220.163.82",
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
	exitCode := run(&stdout, &stderr, []string{"-json"}, dependencies{
		collect: func(context.Context) (networkconfig.Configuration, error) {
			return testConfiguration(), nil
		},
	})

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
	exitCode := run(&stdout, &stderr, []string{"-interface", "wi*"}, dependencies{
		collect: func(context.Context) (networkconfig.Configuration, error) {
			return testConfiguration(), nil
		},
	})

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

func TestRunRejectsUnsupportedStateChangingOption(t *testing.T) {
	t.Parallel()

	called := false
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"/renew"}, dependencies{
		collect: func(context.Context) (networkconfig.Configuration, error) {
			called = true
			return networkconfig.Configuration{}, nil
		},
	})

	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
	if called {
		t.Fatal("collector must not run for an unsupported state-changing option")
	}
	if !strings.Contains(stderr.String(), "no network settings were changed") {
		t.Fatalf("expected safe unsupported-option error, got %q", stderr.String())
	}
}

func TestRunHelpDoesNotCollect(t *testing.T) {
	t.Parallel()

	called := false
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, []string{"/?"}, dependencies{
		collect: func(context.Context) (networkconfig.Configuration, error) {
			called = true
			return networkconfig.Configuration{}, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if called {
		t.Fatal("collector must not run for help")
	}
	if !strings.Contains(stdout.String(), "WINDOWS-COMPATIBLE OPTIONS") {
		t.Fatalf("expected help output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no duplicate help on stderr, got %q", stderr.String())
	}
}

func TestRunPropagatesHelpWriterFailure(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	exitCode := run(failingWriter{}, &stderr, []string{"-h"}, dependencies{
		collect: func(context.Context) (networkconfig.Configuration, error) {
			return networkconfig.Configuration{}, nil
		},
	})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "write output") {
		t.Fatalf("expected help write error, got %q", stderr.String())
	}
}

func TestRunPropagatesTextWriterFailure(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	exitCode := run(failingWriter{}, &stderr, nil, dependencies{
		collect: func(context.Context) (networkconfig.Configuration, error) {
			return testConfiguration(), nil
		},
	})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "write output") {
		t.Fatalf("expected write error, got %q", stderr.String())
	}
}

func TestRunReportsCollectorFailure(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(&stdout, &stderr, nil, dependencies{
		collect: func(context.Context) (networkconfig.Configuration, error) {
			return networkconfig.Configuration{}, errors.New("system configuration unavailable")
		},
	})

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

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("disk full")
}

func testConfiguration() networkconfig.Configuration {
	return networkconfig.Configuration{
		Host: networkconfig.HostInfo{
			HostName:         "macbook",
			PrimaryDNSSuffix: "example.test",
			NodeType:         "Hybrid",
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
				IPv4Addresses: []networkconfig.IPv4Assignment{
					{Address: "192.168.0.20", SubnetMask: "255.255.255.0"},
				},
				DefaultGateways: []string{"192.168.0.1"},
				DHCPServer:      "192.168.0.1",
				DNSServers:      []string{"210.220.163.82", "219.250.36.130"},
				LeaseObtained:   "07/25/2026 16:57:17",
				LeaseExpires:    "07/25/2026 18:57:17",
			},
			{
				Name:            "USB Ethernet",
				Description:     "Ethernet (en5)",
				InterfaceName:   "en5",
				Kind:            networkconfig.AdapterEthernet,
				PhysicalAddress: "11-22-33-44-55-66",
			},
		},
	}
}
