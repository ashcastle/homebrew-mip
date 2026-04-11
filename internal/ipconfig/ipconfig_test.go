package ipconfig

import (
	"bytes"
	"net"
	"strings"
	"testing"
)

func TestParseDefaultGateways(t *testing.T) {
	t.Parallel()

	output := `
Routing tables

Internet:
Destination        Gateway            Flags               Netif Expire
default            192.168.0.1        UGSc                 en0
default            link#18            UCSI                utun4
0.0.0.0            10.0.0.1           UGSc                en5
`

	got := parseDefaultGateways(output)

	if got["en0"] != "192.168.0.1" {
		t.Fatalf("expected en0 gateway to be 192.168.0.1, got %q", got["en0"])
	}

	if got["utun4"] != "N/A" {
		t.Fatalf("expected utun4 gateway to be N/A, got %q", got["utun4"])
	}

	if got["en5"] != "10.0.0.1" {
		t.Fatalf("expected en5 gateway to be 10.0.0.1, got %q", got["en5"])
	}
}

func TestParseDNSServers(t *testing.T) {
	t.Parallel()

	contents := `
search local
nameserver 8.8.8.8
nameserver 1.1.1.1
nameserver 8.8.8.8
`

	got := parseDNSServers(contents)
	want := []string{"8.8.8.8", "1.1.1.1"}

	if len(got) != len(want) {
		t.Fatalf("expected %d DNS servers, got %d (%v)", len(want), len(got), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected DNS server %d to be %q, got %q", i, want[i], got[i])
		}
	}
}

func TestRunJSONOutputHasNoBanner(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(&stdout, &stderr, []string{"-json"}, dependencies{
		snapshots: func() ([]InterfaceSnapshot, error) {
			return []InterfaceSnapshot{
				{
					Interface: net.Interface{Name: "en0", Flags: net.FlagUp, MTU: 1500},
					Addrs: []net.Addr{
						&net.IPNet{IP: net.ParseIP("192.168.0.10"), Mask: net.CIDRMask(24, 32)},
					},
				},
			}, nil
		},
		defaultGateways: func() map[string]string {
			return map[string]string{"en0": "192.168.0.1"}
		},
		dnsServers: func() []string {
			return []string{"1.1.1.1"}
		},
	})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}

	if strings.Contains(stdout.String(), "Network Configuration") {
		t.Fatalf("expected JSON output without banner, got %q", stdout.String())
	}

	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "[") {
		t.Fatalf("expected JSON array output, got %q", stdout.String())
	}
}

func TestRunFailsForUnknownInterface(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(&stdout, &stderr, []string{"-interface", "en9"}, dependencies{
		snapshots: func() ([]InterfaceSnapshot, error) {
			return []InterfaceSnapshot{
				{
					Interface: net.Interface{Name: "en0", Flags: net.FlagUp},
					Addrs: []net.Addr{
						&net.IPNet{IP: net.ParseIP("192.168.0.10"), Mask: net.CIDRMask(24, 32)},
					},
				},
			}, nil
		},
		defaultGateways: func() map[string]string { return map[string]string{} },
		dnsServers:      func() []string { return nil },
	})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}

	if !strings.Contains(stderr.String(), `interface "en9" not found`) {
		t.Fatalf("expected missing interface error, got %q", stderr.String())
	}
}
