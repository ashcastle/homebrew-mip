package networkconfig

import (
	"context"
	"fmt"
)

type Configuration struct {
	Host     HostInfo  `json:"host"`
	Adapters []Adapter `json:"adapters"`
}

type HostInfo struct {
	HostName         string `json:"host_name"`
	PrimaryDNSSuffix string `json:"primary_dns_suffix,omitempty"`
	NodeType         string `json:"node_type"`
	IPRoutingEnabled bool   `json:"ip_routing_enabled"`
	WINSProxyEnabled bool   `json:"wins_proxy_enabled"`
	DHCPv6DUID       string `json:"dhcpv6_duid,omitempty"`
}

type Adapter struct {
	Name                     string           `json:"name"`
	Description              string           `json:"description"`
	InterfaceName            string           `json:"interface_name"`
	Kind                     AdapterKind      `json:"kind"`
	Connected                bool             `json:"connected"`
	PhysicalAddress          string           `json:"physical_address,omitempty"`
	MTU                      int              `json:"mtu,omitempty"`
	DHCPEnabled              bool             `json:"dhcp_enabled"`
	AutoconfigurationEnabled bool             `json:"autoconfiguration_enabled"`
	IPv6Automatic            bool             `json:"ipv6_automatic"`
	ConnectionSpecificSuffix string           `json:"connection_specific_dns_suffix,omitempty"`
	IPv4Addresses            []IPv4Assignment `json:"ipv4_addresses,omitempty"`
	IPv6Addresses            []string         `json:"ipv6_addresses,omitempty"`
	DefaultGateways          []string         `json:"default_gateways,omitempty"`
	DHCPServer               string           `json:"dhcp_server,omitempty"`
	DNSServers               []string         `json:"dns_servers,omitempty"`
	LeaseObtained            string           `json:"lease_obtained,omitempty"`
	LeaseExpires             string           `json:"lease_expires,omitempty"`
	DHCPv6IAID               string           `json:"dhcpv6_iaid,omitempty"`
	Primary                  bool             `json:"primary,omitempty"`
	ServiceOrder             int              `json:"-"`
}

type IPv4Assignment struct {
	Address    string `json:"address"`
	SubnetMask string `json:"subnet_mask"`
}

type AdapterKind string

const (
	AdapterEthernet AdapterKind = "ethernet"
	AdapterWireless AdapterKind = "wireless"
	AdapterTunnel   AdapterKind = "tunnel"
	AdapterBridge   AdapterKind = "bridge"
	AdapterVirtual  AdapterKind = "virtual"
	AdapterUnknown  AdapterKind = "unknown"
)

type Collector interface {
	Collect(context.Context) (Configuration, error)
}

func Collect(ctx context.Context) (Configuration, error) {
	config, err := newPlatformCollector().Collect(ctx)
	if err != nil {
		return Configuration{}, fmt.Errorf("collect network configuration: %w", err)
	}
	return config, nil
}
