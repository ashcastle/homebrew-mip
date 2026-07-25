//go:build darwin

package networkconfig

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	scutilPath       = "/usr/sbin/scutil"
	networksetupPath = "/usr/sbin/networksetup"
	sysctlPath       = "/usr/sbin/sysctl"
)

var serviceIDPattern = regexp.MustCompile(`Setup:/Network/Service/([^/\s]+)`)

type commandRunner func(context.Context, string, []string, string) ([]byte, error)

type interfaceSnapshot struct {
	Interface net.Interface
	Addresses []net.Addr
}

type darwinCollector struct {
	run        commandRunner
	interfaces func() ([]interfaceSnapshot, error)
	hostname   func() (string, error)
}

type scutilRecord map[string][]string

type serviceRecords struct {
	id        string
	root      scutilRecord
	iface     scutilRecord
	setupIPv4 scutilRecord
	setupIPv6 scutilRecord
	stateIPv4 scutilRecord
	stateIPv6 scutilRecord
	dns       scutilRecord
	dhcp      scutilRecord
}

type hardwarePort struct {
	name    string
	address string
}

func newPlatformCollector() Collector {
	return darwinCollector{
		run:        runCommand,
		interfaces: loadInterfaces,
		hostname:   os.Hostname,
	}
}

func (c darwinCollector) Collect(ctx context.Context) (Configuration, error) {
	snapshots, err := c.interfaces()
	if err != nil {
		return Configuration{}, err
	}

	serviceIDs, err := c.serviceIDs(ctx)
	if err != nil {
		return Configuration{}, err
	}

	setupGlobalIPv4, stateGlobalIPv4, globalDNS, services, err := c.loadSystemConfiguration(ctx, serviceIDs)
	if err != nil {
		return Configuration{}, err
	}

	ports := map[string]hardwarePort{}
	if output, runErr := c.run(ctx, networksetupPath, []string{"-listallhardwareports"}, ""); runErr == nil {
		ports = parseHardwarePorts(string(output))
	}

	config := Configuration{
		Host: HostInfo{
			NodeType: "Hybrid",
		},
	}
	if hostname, hostnameErr := c.hostname(); hostnameErr == nil {
		config.Host.HostName = hostname
		if separator := strings.Index(hostname, "."); separator > 0 {
			config.Host.HostName = hostname[:separator]
			suffix := hostname[separator+1:]
			if !strings.EqualFold(suffix, "local") {
				config.Host.PrimaryDNSSuffix = suffix
			}
		}
	}
	if output, runErr := c.run(ctx, sysctlPath, []string{"-n", "net.inet.ip.forwarding"}, ""); runErr == nil {
		config.Host.IPRoutingEnabled = strings.TrimSpace(string(output)) == "1"
	}
	if output, runErr := c.run(ctx, appleIPConfigPath, []string{"getdhcpduid"}, ""); runErr == nil {
		config.Host.DHCPv6DUID = strings.TrimSpace(string(output))
	}

	adapters := make(map[string]*Adapter, len(snapshots)+len(services))
	interfaceIndexes := make(map[string]int, len(snapshots))
	configuredInterfaces := make(map[string]bool, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.Interface.Flags&net.FlagLoopback != 0 {
			continue
		}

		adapter := &Adapter{
			Name:          snapshot.Interface.Name,
			Description:   "Network Adapter " + snapshot.Interface.Name,
			InterfaceName: snapshot.Interface.Name,
			Kind:          inferAdapterKind(snapshot.Interface.Name, "", ""),
			Connected:     snapshot.Interface.Flags&net.FlagUp != 0 && len(snapshot.Addresses) > 0,
			MTU:           snapshot.Interface.MTU,
			ServiceOrder:  int(^uint(0) >> 1),
		}
		if len(snapshot.Interface.HardwareAddr) > 0 {
			adapter.PhysicalAddress = strings.ToUpper(strings.ReplaceAll(snapshot.Interface.HardwareAddr.String(), ":", "-"))
		}
		addSnapshotAddresses(adapter, snapshot.Addresses)
		adapters[adapter.InterfaceName] = adapter
		interfaceIndexes[adapter.InterfaceName] = snapshot.Interface.Index
	}

	for device, port := range ports {
		adapter := ensureAdapter(adapters, device)
		configuredInterfaces[device] = true
		adapter.Name = port.name
		adapter.Description = port.name
		if adapter.PhysicalAddress == "" && port.address != "" {
			adapter.PhysicalAddress = strings.ToUpper(strings.ReplaceAll(port.address, ":", "-"))
		}
		adapter.Kind = inferAdapterKind(device, port.name, "")
	}

	serviceOrder := indexValues(setupGlobalIPv4.values("ServiceOrder"))
	primaryService := stateGlobalIPv4.first("PrimaryService")
	primaryInterface := stateGlobalIPv4.first("PrimaryInterface")

	for _, service := range services {
		interfaceName := firstNonEmpty(
			service.iface.first("DeviceName"),
			service.stateIPv4.first("InterfaceName"),
			service.stateIPv4.first("ConfirmedInterfaceName"),
			service.stateIPv6.first("InterfaceName"),
		)
		if interfaceName == "" {
			continue
		}

		adapter := ensureAdapter(adapters, interfaceName)
		configuredInterfaces[interfaceName] = true
		serviceName := firstNonEmpty(service.root.first("UserDefinedName"), service.iface.first("UserDefinedName"))
		if serviceName != "" {
			adapter.Name = serviceName
		}

		hardware := service.iface.first("Hardware")
		interfaceType := service.iface.first("Type")
		adapter.Kind = inferAdapterKind(interfaceName, adapter.Name, hardware)
		adapter.Description = adapterDescription(adapter, hardware, interfaceType)
		configMethod := service.setupIPv4.first("ConfigMethod")
		adapter.DHCPEnabled = strings.EqualFold(configMethod, "DHCP")
		adapter.AutoconfigurationEnabled = adapter.DHCPEnabled ||
			strings.EqualFold(configMethod, "INFORM") ||
			strings.EqualFold(configMethod, "LinkLocal")
		adapter.IPv6Automatic = strings.EqualFold(service.setupIPv6.first("ConfigMethod"), "Automatic")
		adapter.ConnectionSpecificSuffix = firstNonEmpty(
			service.dns.first("DomainName"),
			firstValue(service.dns.values("SearchDomains")),
		)
		adapter.LeaseObtained = service.dhcp.first("LeaseStartTime")
		adapter.LeaseExpires = service.dhcp.first("LeaseExpirationTime")
		adapter.DHCPServer = decodeIPv4Data(service.dhcp.first("Option_54"))
		if adapter.IPv6Automatic {
			if output, runErr := c.run(ctx, appleIPConfigPath, []string{"getdhcpiaid", interfaceName}, ""); runErr == nil {
				adapter.DHCPv6IAID = strings.TrimSpace(string(output))
			}
		}
		adapter.ServiceOrder = valueIndex(serviceOrder, service.id)
		adapter.Primary = service.id == primaryService || interfaceName == primaryInterface

		mergeIPv4State(adapter, service.stateIPv4)
		mergeIPv6State(adapter, service.stateIPv6)
		adapter.DefaultGateways = uniqueStrings(append(adapter.DefaultGateways,
			service.stateIPv4.first("Router"),
			service.stateIPv6.first("Router"),
		))
		adapter.DNSServers = uniqueStrings(append(adapter.DNSServers, service.dns.values("ServerAddresses")...))
		adapter.Connected = adapter.Connected || len(service.stateIPv4.values("Addresses")) > 0 ||
			len(service.stateIPv6.values("Addresses")) > 0
	}

	if primary := adapters[primaryInterface]; primary != nil {
		primary.Primary = true
		primary.DefaultGateways = uniqueStrings(append(primary.DefaultGateways, stateGlobalIPv4.first("Router")))
		if len(primary.DNSServers) == 0 {
			primary.DNSServers = uniqueStrings(globalDNS.values("ServerAddresses"))
		}
		if primary.ConnectionSpecificSuffix != "" && config.Host.PrimaryDNSSuffix == "" {
			config.Host.PrimaryDNSSuffix = primary.ConnectionSpecificSuffix
		}
	}

	config.Adapters = make([]Adapter, 0, len(adapters))
	for _, adapter := range adapters {
		normalizeAdapter(adapter, interfaceIndexes[adapter.InterfaceName])
		if includeAdapter(*adapter, configuredInterfaces[adapter.InterfaceName]) {
			config.Adapters = append(config.Adapters, *adapter)
		}
	}

	sort.SliceStable(config.Adapters, func(i, j int) bool {
		left, right := config.Adapters[i], config.Adapters[j]
		if left.Primary != right.Primary {
			return left.Primary
		}
		if left.Connected != right.Connected {
			return left.Connected
		}
		if left.ServiceOrder != right.ServiceOrder {
			return left.ServiceOrder < right.ServiceOrder
		}
		leftIndex := interfaceIndexes[left.InterfaceName]
		rightIndex := interfaceIndexes[right.InterfaceName]
		if leftIndex != rightIndex {
			return leftIndex < rightIndex
		}
		return left.InterfaceName < right.InterfaceName
	})

	return config, nil
}

func (c darwinCollector) serviceIDs(ctx context.Context) ([]string, error) {
	output, err := c.run(ctx, scutilPath, nil, "list Setup:/Network/Service/.*\nquit\n")
	if err != nil {
		return nil, fmt.Errorf("list System Configuration services: %w", err)
	}
	return parseServiceIDs(string(output)), nil
}

func (c darwinCollector) loadSystemConfiguration(ctx context.Context, serviceIDs []string) (scutilRecord, scutilRecord, scutilRecord, []serviceRecords, error) {
	keys := []string{
		"Setup:/Network/Global/IPv4",
		"State:/Network/Global/IPv4",
		"State:/Network/Global/DNS",
	}
	for _, id := range serviceIDs {
		prefix := "/Network/Service/" + id
		keys = append(keys,
			"Setup:"+prefix,
			"Setup:"+prefix+"/Interface",
			"Setup:"+prefix+"/IPv4",
			"Setup:"+prefix+"/IPv6",
			"State:"+prefix+"/IPv4",
			"State:"+prefix+"/IPv6",
			"State:"+prefix+"/DNS",
			"State:"+prefix+"/DHCP",
		)
	}

	var input strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&input, "show %s\n", key)
	}
	input.WriteString("quit\n")

	output, err := c.run(ctx, scutilPath, nil, input.String())
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read System Configuration: %w", err)
	}

	records, err := parseSCUtilResponses(string(output), len(keys))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	services := make([]serviceRecords, 0, len(serviceIDs))
	offset := 3
	for index, id := range serviceIDs {
		start := offset + index*8
		services = append(services, serviceRecords{
			id:        id,
			root:      records[start],
			iface:     records[start+1],
			setupIPv4: records[start+2],
			setupIPv6: records[start+3],
			stateIPv4: records[start+4],
			stateIPv6: records[start+5],
			dns:       records[start+6],
			dhcp:      records[start+7],
		})
	}

	return records[0], records[1], records[2], services, nil
}

func runCommand(ctx context.Context, path string, args []string, stdin string) ([]byte, error) {
	command := exec.CommandContext(ctx, path, args...)
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, errors.New(message)
	}
	return stdout.Bytes(), nil
}

func loadInterfaces() ([]interfaceSnapshot, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("get interfaces: %w", err)
	}

	snapshots := make([]interfaceSnapshot, 0, len(interfaces))
	for _, iface := range interfaces {
		addresses, addressErr := iface.Addrs()
		if addressErr != nil {
			return nil, fmt.Errorf("inspect interface %s: %w", iface.Name, addressErr)
		}
		snapshots = append(snapshots, interfaceSnapshot{Interface: iface, Addresses: addresses})
	}
	return snapshots, nil
}

func parseServiceIDs(output string) []string {
	matches := serviceIDPattern.FindAllStringSubmatch(output, -1)
	seen := make(map[string]struct{}, len(matches))
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		if _, exists := seen[match[1]]; exists {
			continue
		}
		seen[match[1]] = struct{}{}
		ids = append(ids, match[1])
	}
	sort.Strings(ids)
	return ids
}

func parseSCUtilResponses(output string, expected int) ([]scutilRecord, error) {
	lines := strings.Split(output, "\n")
	records := make([]scutilRecord, 0, expected)

	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		switch {
		case line == "No such key":
			records = append(records, scutilRecord{})
		case strings.HasPrefix(line, "<dictionary> {"):
			start := index
			depth := strings.Count(line, "{") - strings.Count(line, "}")
			for depth > 0 && index+1 < len(lines) {
				index++
				depth += strings.Count(lines[index], "{") - strings.Count(lines[index], "}")
			}
			records = append(records, parseSCUtilDictionary(lines[start:index+1]))
		}
	}

	if len(records) != expected {
		return nil, fmt.Errorf("parse System Configuration: expected %d records, got %d", expected, len(records))
	}
	return records, nil
}

func parseSCUtilDictionary(lines []string) scutilRecord {
	record := make(scutilRecord)
	depth := 0
	arrayKey := ""

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		if depth == 1 {
			key, value, ok := splitSCUtilEntry(line)
			if ok {
				if strings.HasPrefix(value, "<array> {") {
					arrayKey = key
					record[key] = nil
				} else {
					record[key] = []string{cleanSCUtilValue(value)}
				}
			}
		} else if depth == 2 && arrayKey != "" {
			_, value, ok := splitSCUtilEntry(line)
			if ok && !strings.Contains(value, "<dictionary>") {
				record[arrayKey] = append(record[arrayKey], cleanSCUtilValue(value))
			}
		}

		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth < 2 {
			arrayKey = ""
		}
	}
	return record
}

func splitSCUtilEntry(line string) (string, string, bool) {
	parts := strings.SplitN(line, " : ", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func cleanSCUtilValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "<data>")
	return strings.TrimSpace(value)
}

func parseHardwarePorts(output string) map[string]hardwarePort {
	ports := make(map[string]hardwarePort)
	var name string
	var device string
	var address string

	flush := func() {
		if device != "" {
			ports[device] = hardwarePort{name: name, address: address}
		}
		name, device, address = "", "", ""
	}

	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "Hardware Port: "):
			name = strings.TrimPrefix(line, "Hardware Port: ")
		case strings.HasPrefix(line, "Device: "):
			device = strings.TrimPrefix(line, "Device: ")
		case strings.HasPrefix(line, "Ethernet Address: "):
			address = strings.TrimPrefix(line, "Ethernet Address: ")
		}
	}
	flush()
	return ports
}

func addSnapshotAddresses(adapter *Adapter, addresses []net.Addr) {
	for _, address := range addresses {
		ip, network, err := net.ParseCIDR(address.String())
		if err != nil {
			continue
		}
		if ipv4 := ip.To4(); ipv4 != nil {
			mask := net.IP(network.Mask).String()
			adapter.IPv4Addresses = appendUniqueIPv4(adapter.IPv4Addresses, IPv4Assignment{
				Address:    ipv4.String(),
				SubnetMask: mask,
			})
			continue
		}
		if ip.To16() != nil {
			adapter.IPv6Addresses = append(adapter.IPv6Addresses, ip.String())
		}
	}
}

func mergeIPv4State(adapter *Adapter, state scutilRecord) {
	addresses := state.values("Addresses")
	masks := state.values("SubnetMasks")
	for index, address := range addresses {
		mask := ""
		if index < len(masks) {
			mask = masks[index]
		}
		adapter.IPv4Addresses = appendUniqueIPv4(adapter.IPv4Addresses, IPv4Assignment{
			Address:    address,
			SubnetMask: mask,
		})
	}
}

func mergeIPv6State(adapter *Adapter, state scutilRecord) {
	adapter.IPv6Addresses = uniqueStrings(append(adapter.IPv6Addresses, state.values("Addresses")...))
}

func ensureAdapter(adapters map[string]*Adapter, interfaceName string) *Adapter {
	if adapter := adapters[interfaceName]; adapter != nil {
		return adapter
	}
	adapter := &Adapter{
		Name:          interfaceName,
		Description:   "Network Adapter " + interfaceName,
		InterfaceName: interfaceName,
		Kind:          inferAdapterKind(interfaceName, "", ""),
		ServiceOrder:  int(^uint(0) >> 1),
	}
	adapters[interfaceName] = adapter
	return adapter
}

func adapterDescription(adapter *Adapter, hardware, interfaceType string) string {
	switch {
	case hardware != "":
		return hardware + " (" + adapter.InterfaceName + ")"
	case interfaceType != "":
		return interfaceType + " (" + adapter.InterfaceName + ")"
	case adapter.Description != "":
		return adapter.Description
	default:
		return "Network Adapter " + adapter.InterfaceName
	}
}

func inferAdapterKind(interfaceName, displayName, hardware string) AdapterKind {
	value := strings.ToLower(strings.Join([]string{interfaceName, displayName, hardware}, " "))
	switch {
	case strings.Contains(value, "wi-fi"), strings.Contains(value, "wifi"), strings.Contains(value, "airport"):
		return AdapterWireless
	case strings.HasPrefix(interfaceName, "utun"), strings.Contains(value, "tunnel"), strings.Contains(value, "vpn"):
		return AdapterTunnel
	case strings.HasPrefix(interfaceName, "bridge"), strings.Contains(value, "bridge"):
		return AdapterBridge
	case strings.HasPrefix(interfaceName, "en"), strings.Contains(value, "ethernet"):
		return AdapterEthernet
	case strings.HasPrefix(interfaceName, "awdl"), strings.HasPrefix(interfaceName, "llw"):
		return AdapterVirtual
	default:
		return AdapterUnknown
	}
}

func normalizeAdapter(adapter *Adapter, interfaceIndex int) {
	adapter.IPv4Addresses = uniqueIPv4(adapter.IPv4Addresses)
	adapter.IPv6Addresses = uniqueStrings(adapter.IPv6Addresses)
	if interfaceIndex > 0 {
		for index, address := range adapter.IPv6Addresses {
			if strings.HasPrefix(strings.ToLower(address), "fe80:") && !strings.Contains(address, "%") {
				adapter.IPv6Addresses[index] = address + "%" + strconv.Itoa(interfaceIndex)
			}
		}
	}
	adapter.DefaultGateways = filterGateways(*adapter)
	adapter.DNSServers = uniqueStrings(adapter.DNSServers)
	adapter.ConnectionSpecificSuffix = strings.TrimSuffix(adapter.ConnectionSpecificSuffix, ".")
	if adapter.Name == "" {
		adapter.Name = adapter.InterfaceName
	}
	if adapter.Description == "" {
		adapter.Description = "Network Adapter " + adapter.InterfaceName
	}
}

func includeAdapter(adapter Adapter, configured bool) bool {
	if adapter.InterfaceName == "" || strings.HasPrefix(adapter.InterfaceName, "lo") {
		return false
	}
	if configured {
		return true
	}
	if len(adapter.IPv4Addresses) > 0 {
		return true
	}
	for _, address := range adapter.IPv6Addresses {
		if !strings.HasPrefix(strings.ToLower(address), "fe80:") {
			return true
		}
	}
	return false
}

func filterGateways(adapter Adapter) []string {
	gateways := uniqueStrings(adapter.DefaultGateways)
	if adapter.Kind != AdapterTunnel {
		return gateways
	}

	filtered := make([]string, 0, len(gateways))
	for _, gateway := range gateways {
		gatewayIP := net.ParseIP(strings.SplitN(gateway, "%", 2)[0])
		if gatewayIP == nil {
			filtered = append(filtered, gateway)
			continue
		}

		onLink := false
		for _, assignment := range adapter.IPv4Addresses {
			if gatewayIP.Equal(net.ParseIP(assignment.Address)) {
				onLink = true
				break
			}
		}
		for _, address := range adapter.IPv6Addresses {
			addressIP := net.ParseIP(strings.SplitN(address, "%", 2)[0])
			if sameIPv6Prefix(addressIP, gatewayIP, 64) {
				onLink = true
				break
			}
		}
		if !onLink {
			filtered = append(filtered, gateway)
		}
	}
	return filtered
}

func sameIPv6Prefix(left, right net.IP, bits int) bool {
	left = left.To16()
	right = right.To16()
	if left == nil || right == nil || left.To4() != nil || right.To4() != nil {
		return false
	}
	mask := net.CIDRMask(bits, 128)
	return left.Mask(mask).Equal(right.Mask(mask))
}

func appendUniqueIPv4(values []IPv4Assignment, candidate IPv4Assignment) []IPv4Assignment {
	if candidate.Address == "" {
		return values
	}
	for index, value := range values {
		if value.Address != candidate.Address {
			continue
		}
		if values[index].SubnetMask == "" {
			values[index].SubnetMask = candidate.SubnetMask
		}
		return values
	}
	return append(values, candidate)
}

func uniqueIPv4(values []IPv4Assignment) []IPv4Assignment {
	result := make([]IPv4Assignment, 0, len(values))
	for _, value := range values {
		result = appendUniqueIPv4(result, value)
	}
	return result
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "0.0.0.0" || value == "::" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func decodeIPv4Data(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(value), "0x")
	if len(value) < 8 {
		return ""
	}
	data, err := hex.DecodeString(value[:8])
	if err != nil || len(data) != net.IPv4len {
		return ""
	}
	return net.IP(data).String()
}

func indexValues(values []string) map[string]int {
	indexes := make(map[string]int, len(values))
	for index, value := range values {
		indexes[value] = index
	}
	return indexes
}

func valueIndex(indexes map[string]int, value string) int {
	if index, exists := indexes[value]; exists {
		return index
	}
	return int(^uint(0) >> 1)
}

func firstValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (r scutilRecord) first(key string) string {
	return firstValue(r.values(key))
}

func (r scutilRecord) values(key string) []string {
	if r == nil {
		return nil
	}
	return r[key]
}
