//go:build darwin

package networkconfig

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestParseSCUtilResponses(t *testing.T) {
	t.Parallel()

	output := `<dictionary> {
  PrimaryInterface : en0
  ServiceOrder : <array> {
    0 : SERVICE-B
    1 : SERVICE-A
  }
  AdditionalRoutes : <array> {
    0 : <dictionary> {
      DestinationAddress : 192.168.0.20
    }
  }
}
  No such key
<dictionary> {
  Option_54 : <data> 0xc0a80001
}
`

	records, err := parseSCUtilResponses(output, 3)
	if err != nil {
		t.Fatalf("parseSCUtilResponses returned error: %v", err)
	}
	if got := records[0].first("PrimaryInterface"); got != "en0" {
		t.Fatalf("PrimaryInterface=%q, want en0", got)
	}
	if got := records[0].values("ServiceOrder"); len(got) != 2 || got[0] != "SERVICE-B" || got[1] != "SERVICE-A" {
		t.Fatalf("unexpected ServiceOrder: %#v", got)
	}
	if len(records[1]) != 0 {
		t.Fatalf("missing key should produce an empty record, got %#v", records[1])
	}
	if got := records[2].first("Option_54"); got != "0xc0a80001" {
		t.Fatalf("Option_54=%q, want 0xc0a80001", got)
	}
}

func TestParseServiceIDsIsUniqueAndStable(t *testing.T) {
	t.Parallel()

	output := `
  subKey [0] = Setup:/Network/Service/SERVICE-B/IPv4
  subKey [1] = Setup:/Network/Service/SERVICE-A
  subKey [2] = Setup:/Network/Service/SERVICE-B/Interface
`
	got := parseServiceIDs(output)
	if len(got) != 2 || got[0] != "SERVICE-A" || got[1] != "SERVICE-B" {
		t.Fatalf("unexpected service IDs: %#v", got)
	}
}

func TestParseHardwarePorts(t *testing.T) {
	t.Parallel()

	output := `Hardware Port: Wi-Fi
Device: en0
Ethernet Address: aa:bb:cc:dd:ee:ff

Hardware Port: Thunderbolt Bridge
Device: bridge0
Ethernet Address: 11:22:33:44:55:66
`
	ports := parseHardwarePorts(output)
	if got := ports["en0"]; got.name != "Wi-Fi" || got.address != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("unexpected en0 hardware port: %#v", got)
	}
	if got := ports["bridge0"].name; got != "Thunderbolt Bridge" {
		t.Fatalf("unexpected bridge port name: %q", got)
	}
}

func TestDarwinCollectorBuildsWindowsModel(t *testing.T) {
	t.Parallel()

	runner := func(_ context.Context, path string, _ []string, stdin string) ([]byte, error) {
		switch {
		case path == networksetupPath:
			return []byte("Hardware Port: Wi-Fi\nDevice: en0\nEthernet Address: aa:bb:cc:dd:ee:ff\n"), nil
		case strings.HasPrefix(stdin, "list "):
			return []byte("subKey [0] = Setup:/Network/Service/SERVICE-1/IPv4\n"), nil
		default:
			return []byte(scutilCollectorFixture), nil
		}
	}

	collector := darwinCollector{
		run: runner,
		interfaces: func() ([]interfaceSnapshot, error) {
			return []interfaceSnapshot{
				{
					Interface: net.Interface{
						Index:        14,
						MTU:          1500,
						Name:         "en0",
						HardwareAddr: net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
						Flags:        net.FlagUp,
					},
					Addresses: []net.Addr{
						&net.IPNet{IP: net.ParseIP("192.168.0.20"), Mask: net.CIDRMask(24, 32)},
					},
				},
			}, nil
		},
		hostname: func() (string, error) { return "macbook", nil },
	}

	config, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if config.Host.HostName != "macbook" {
		t.Fatalf("HostName=%q, want macbook", config.Host.HostName)
	}
	if len(config.Adapters) != 1 {
		t.Fatalf("expected one adapter, got %#v", config.Adapters)
	}

	adapter := config.Adapters[0]
	if adapter.Name != "Wi-Fi" || adapter.Kind != AdapterWireless || !adapter.Primary {
		t.Fatalf("unexpected adapter identity: %#v", adapter)
	}
	if !adapter.DHCPEnabled || adapter.DHCPServer != "192.168.0.1" {
		t.Fatalf("unexpected DHCP data: %#v", adapter)
	}
	if len(adapter.DNSServers) != 2 || adapter.DNSServers[0] != "1.1.1.1" {
		t.Fatalf("unexpected DNS servers: %#v", adapter.DNSServers)
	}
	if len(adapter.DefaultGateways) != 1 || adapter.DefaultGateways[0] != "192.168.0.1" {
		t.Fatalf("unexpected gateways: %#v", adapter.DefaultGateways)
	}
}

func TestDecodeIPv4Data(t *testing.T) {
	t.Parallel()

	if got := decodeIPv4Data("0xc0a80001"); got != "192.168.0.1" {
		t.Fatalf("decodeIPv4Data=%q, want 192.168.0.1", got)
	}
	if got := decodeIPv4Data("invalid"); got != "" {
		t.Fatalf("invalid DHCP data decoded as %q", got)
	}
}

const scutilCollectorFixture = `<dictionary> {
  ServiceOrder : <array> {
    0 : SERVICE-1
  }
}
<dictionary> {
  PrimaryInterface : en0
  PrimaryService : SERVICE-1
  Router : 192.168.0.1
}
<dictionary> {
  ServerAddresses : <array> {
    0 : 9.9.9.9
  }
}
<dictionary> {
  UserDefinedName : Wi-Fi
}
<dictionary> {
  DeviceName : en0
  Hardware : AirPort
  Type : Ethernet
  UserDefinedName : Wi-Fi
}
<dictionary> {
  ConfigMethod : DHCP
}
<dictionary> {
  Addresses : <array> {
    0 : 192.168.0.20
  }
  InterfaceName : en0
  Router : 192.168.0.1
  SubnetMasks : <array> {
    0 : 255.255.255.0
  }
}
  No such key
<dictionary> {
  DomainName : example.test
  ServerAddresses : <array> {
    0 : 1.1.1.1
    1 : 8.8.8.8
  }
}
<dictionary> {
  LeaseExpirationTime : 07/25/2026 18:57:17
  LeaseStartTime : 07/25/2026 16:57:17
  Option_54 : <data> 0xc0a80001
}
`
