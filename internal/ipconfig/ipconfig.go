package ipconfig

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sort"
	"strings"
)

const CommandName = "ipconfig"

type AdapterInfo struct {
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	ConnectionSuffix string   `json:"connection_suffix,omitempty"`
	IPv4Address      string   `json:"ipv4_address"`
	SubnetMask       string   `json:"subnet_mask"`
	DefaultGateway   string   `json:"default_gateway"`
	IPv6Addresses    []string `json:"ipv6_addresses,omitempty"`
	PhysicalAddress  string   `json:"physical_address,omitempty"`
	MTU              int      `json:"mtu,omitempty"`
	DNSServers       []string `json:"dns_servers,omitempty"`
}

type InterfaceSnapshot struct {
	Interface net.Interface
	Addrs     []net.Addr
}

type dependencies struct {
	snapshots       func() ([]InterfaceSnapshot, error)
	defaultGateways func() map[string]string
	dnsServers      func() []string
}

func Run(stdout, stderr io.Writer, args []string) int {
	return run(stdout, stderr, args, defaultDependencies())
}

func run(stdout, stderr io.Writer, args []string, deps dependencies) int {
	fs := flag.NewFlagSet(CommandName, flag.ContinueOnError)
	fs.SetOutput(stderr)

	interfaceName := fs.String("interface", "", "Display only a specific interface")
	jsonOutput := fs.Bool("json", false, "Output machine-readable JSON")
	showAll := fs.Bool("all", false, "Show detailed interface information")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s [options]\n\nOptions:\n", CommandName)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "%s: unexpected arguments: %s\n\n", CommandName, strings.Join(fs.Args(), " "))
		fs.Usage()
		return 2
	}

	adapters, err := collectAdapters(*interfaceName, *showAll, deps)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", CommandName, err)
		return 1
	}

	if *jsonOutput {
		if err := writeJSON(stdout, adapters); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", CommandName, err)
			return 1
		}
		return 0
	}

	writeText(stdout, adapters, *showAll)
	return 0
}

func defaultDependencies() dependencies {
	return dependencies{
		snapshots:       loadInterfaceSnapshots,
		defaultGateways: getDefaultGateways,
		dnsServers:      getDNSServers,
	}
}

func loadInterfaceSnapshots() ([]InterfaceSnapshot, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("get interfaces: %w", err)
	}

	snapshots := make([]InterfaceSnapshot, 0, len(ifaces))
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("inspect interface %s: %w", iface.Name, err)
		}
		snapshots = append(snapshots, InterfaceSnapshot{
			Interface: iface,
			Addrs:     addrs,
		})
	}

	return snapshots, nil
}

func collectAdapters(interfaceName string, showAll bool, deps dependencies) ([]AdapterInfo, error) {
	snapshots, err := deps.snapshots()
	if err != nil {
		return nil, err
	}

	defaultGateways := deps.defaultGateways()
	dnsServers := deps.dnsServers()

	adapters := make([]AdapterInfo, 0, len(snapshots))
	seenInterface := false

	for _, snapshot := range snapshots {
		if interfaceName != "" && snapshot.Interface.Name != interfaceName {
			continue
		}

		if interfaceName != "" {
			seenInterface = true
		}

		if snapshot.Interface.Flags&net.FlagUp == 0 {
			continue
		}

		adapter, err := buildAdapterInfo(snapshot, defaultGateways, showAll, dnsServers)
		if err != nil {
			return nil, err
		}
		if adapter != nil {
			adapters = append(adapters, *adapter)
		}
	}

	sort.Slice(adapters, func(i, j int) bool {
		return adapters[i].Name < adapters[j].Name
	})

	if interfaceName == "" {
		return adapters, nil
	}

	if !seenInterface {
		return nil, fmt.Errorf("interface %q not found", interfaceName)
	}

	if len(adapters) == 0 {
		return nil, fmt.Errorf("interface %q has no active IPv4 configuration", interfaceName)
	}

	return adapters, nil
}

func buildAdapterInfo(snapshot InterfaceSnapshot, defaultGateways map[string]string, showAll bool, dnsServers []string) (*AdapterInfo, error) {
	var ipv4Address string
	var subnetMask string
	var ipv6Addresses []string

	for _, addr := range snapshot.Addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		if ipv4 := ipNet.IP.To4(); ipv4 != nil {
			if ipv4Address == "" {
				ipv4Address = ipv4.String()
				subnetMask = net.IP(ipNet.Mask).String()
			}
			continue
		}

		if ipNet.IP.To16() != nil {
			ipv6Addresses = append(ipv6Addresses, ipNet.IP.String())
		}
	}

	if ipv4Address == "" && snapshot.Interface.Flags&net.FlagLoopback == 0 {
		return nil, nil
	}

	adapter := &AdapterInfo{
		Name:           snapshot.Interface.Name,
		Type:           adapterType(snapshot.Interface),
		IPv4Address:    ipv4Address,
		SubnetMask:     subnetMask,
		DefaultGateway: defaultGatewayForInterface(snapshot.Interface.Name, defaultGateways),
	}

	if showAll {
		if len(ipv6Addresses) > 0 {
			adapter.IPv6Addresses = uniqueStrings(ipv6Addresses)
		}
		if len(snapshot.Interface.HardwareAddr) > 0 {
			adapter.PhysicalAddress = snapshot.Interface.HardwareAddr.String()
		}
		if snapshot.Interface.MTU > 0 {
			adapter.MTU = snapshot.Interface.MTU
		}
		if len(dnsServers) > 0 {
			adapter.DNSServers = uniqueStrings(dnsServers)
		}
	}

	return adapter, nil
}

func adapterType(iface net.Interface) string {
	if iface.Flags&net.FlagLoopback != 0 {
		return "Loopback adapter " + iface.Name
	}

	switch {
	case strings.HasPrefix(iface.Name, "en"):
		return "Ethernet adapter " + iface.Name
	case strings.HasPrefix(iface.Name, "bridge"):
		return "Bridge adapter " + iface.Name
	case strings.HasPrefix(iface.Name, "awdl"):
		return "Wireless Direct Link adapter " + iface.Name
	case strings.HasPrefix(iface.Name, "llw"):
		return "Low-latency Wi-Fi adapter " + iface.Name
	case strings.HasPrefix(iface.Name, "utun"):
		return "Tunnel adapter " + iface.Name
	default:
		return "Network adapter " + iface.Name
	}
}

func defaultGatewayForInterface(name string, gateways map[string]string) string {
	if gateway := gateways[name]; gateway != "" {
		return gateway
	}
	return "N/A"
}

func getDefaultGateways() map[string]string {
	output, err := exec.Command("netstat", "-rn", "-f", "inet").Output()
	if err != nil {
		return map[string]string{}
	}
	return parseDefaultGateways(string(output))
}

func parseDefaultGateways(output string) map[string]string {
	defaultGateways := make(map[string]string)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "default") && !strings.HasPrefix(line, "0.0.0.0") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		gateway := fields[1]
		ifaceName := fields[len(fields)-1]
		if isPlaceholderGateway(gateway) {
			defaultGateways[ifaceName] = "N/A"
			continue
		}

		defaultGateways[ifaceName] = gateway
	}

	return defaultGateways
}

func isPlaceholderGateway(gateway string) bool {
	return strings.HasPrefix(gateway, "link#") || gateway == "lo0" || strings.HasPrefix(gateway, "utun")
}

func getDNSServers() []string {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	return parseDNSServers(string(data))
}

func parseDNSServers(contents string) []string {
	servers := make([]string, 0)

	for _, line := range strings.Split(contents, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "nameserver") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 {
			servers = append(servers, fields[1])
		}
	}

	return uniqueStrings(servers)
}

func writeJSON(w io.Writer, adapters []AdapterInfo) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(adapters)
}

func writeText(w io.Writer, adapters []AdapterInfo, showAll bool) {
	fmt.Fprintf(w, "%s: Network Configuration\n\n", CommandName)

	if len(adapters) == 0 {
		fmt.Fprintln(w, "No active interfaces with IPv4 addresses found.")
		return
	}

	for _, adapter := range adapters {
		fmt.Fprintf(w, "%s:\n", adapter.Type)
		if showAll {
			if len(adapter.IPv6Addresses) > 0 {
				fmt.Fprintf(w, "   IPv6 Addresses . . . . . . . . . . : %s\n", strings.Join(adapter.IPv6Addresses, ", "))
			}
			if adapter.PhysicalAddress != "" {
				fmt.Fprintf(w, "   Physical Address . . . . . . . . . : %s\n", adapter.PhysicalAddress)
			}
			if adapter.MTU > 0 {
				fmt.Fprintf(w, "   MTU . . . . . . . . . . . . . . . . : %d\n", adapter.MTU)
			}
			if len(adapter.DNSServers) > 0 {
				fmt.Fprintf(w, "   DNS Servers . . . . . . . . . . . . : %s\n", strings.Join(adapter.DNSServers, ", "))
			}
		}
		fmt.Fprintf(w, "   IPv4 Address . . . . . . . . . . . : %s\n", valueOrNA(adapter.IPv4Address))
		fmt.Fprintf(w, "   Subnet Mask . . . . . . . . . . . . : %s\n", valueOrNA(adapter.SubnetMask))
		fmt.Fprintf(w, "   Default Gateway . . . . . . . . . . : %s\n\n", valueOrNA(adapter.DefaultGateway))
	}
}

func valueOrNA(value string) string {
	if value == "" {
		return "N/A"
	}
	return value
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
