package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"
)

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

func getDefaultGateways() map[string]string {
	defaultGateways := make(map[string]string)
	out, err := exec.Command("netstat", "-rn").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting default gateways: %v\n", err)
		return defaultGateways
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "default") || strings.HasPrefix(line, "0.0.0.0") {
			fields := strings.Fields(line)
			if len(fields) >= 5 { // 수정: 필드 개수를 5개 이상으로 확인
				ifaceName := fields[len(fields)-1]
				gateway := fields[1]
				defaultGateways[ifaceName] = gateway
			} else if len(fields) >= 4 { // 수정 : 필드 개수가 4개일 때
				ifaceName := fields[len(fields)-1]
				gateway := fields[1]
				if gateway == "link#" || gateway == "lo0" || strings.HasPrefix(gateway, "utun") {
					defaultGateways[ifaceName] = "N/A"
				} else {
					defaultGateways[ifaceName] = gateway
				}
			}
		}
	}
	return defaultGateways
}

func getDNSServers() []string {
	data, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading /etc/resolv.conf: %v\n", err)
		return nil
	}
	var servers []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "nameserver") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				servers = append(servers, fields[1])
			}
		}
	}
	return servers
}

func getAdapterType(iface net.Interface) string {
	if iface.Flags&net.FlagLoopback != 0 {
		return "Loopback Pseudo-Interface 1"
	}

	if strings.HasPrefix(iface.Name, "en") {
		return "Ethernet adapter " + iface.Name
	}

	if strings.HasPrefix(iface.Name, "awdl") || strings.HasPrefix(iface.Name, "utun") {
		return "Wi-Fi adapter " + iface.Name
	}

	return "Unknown adapter " + iface.Name
}

func getAdapterInfo(iface net.Interface, defaultGateways map[string]string, showAll bool, dnsServers []string) *AdapterInfo {
	addrs, err := iface.Addrs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting addresses for interface %s: %v\n", iface.Name, err)
		return nil
	}

	var ipAddr, mask string
	var ipv6Addrs []string
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			if ipNet.IP.To4() != nil {
				ipAddr = ipNet.IP.String()
				mask = net.IP(ipNet.Mask).String()
			} else if ipNet.IP.To16() != nil {
				ipv6Addrs = append(ipv6Addrs, ipNet.IP.String())
			}
		}
	}
	if ipAddr == "" && iface.Flags&net.FlagLoopback == 0 {
		return nil
	}

	dg := defaultGateways[iface.Name]
	if dg == "" {
		dg = "N/A"
	}
	if iface.Flags&net.FlagLoopback != 0 && dg == "" {
		dg = "N/A"
	}
	adapter := &AdapterInfo{
		Name:           iface.Name,
		Type:           getAdapterType(iface),
		IPv4Address:    ipAddr,
		SubnetMask:     mask,
		DefaultGateway: dg,
	}

	if showAll {
		adapter.PhysicalAddress = iface.HardwareAddr.String()
		adapter.MTU = iface.MTU
		adapter.IPv6Addresses = ipv6Addrs
		adapter.DNSServers = dnsServers
	}

	return adapter
}

func printAdapterInfo(adapter *AdapterInfo, showAll bool) {
	fmt.Printf("%s:\n", adapter.Type)
	//fmt.Printf("   Connection-specific DNS Suffix . : %s\n", )
	if showAll {
		if len(adapter.IPv6Addresses) > 0 {
			fmt.Printf("   Link-local IPv6 Address . . . . . : %s\n", strings.Join(adapter.IPv6Addresses, ", "))
		}
		fmt.Printf("   Physical Address. . . . . . . . : %s\n", adapter.PhysicalAddress)
		fmt.Printf("   MTU . . . . . . . . . . . . . . . : %d\n", adapter.MTU)
		if len(adapter.DNSServers) > 0 {
			fmt.Printf("   DNS Servers . . . . . . . . . . . : %s\n", strings.Join(adapter.DNSServers, ", "))
		}
	}
	fmt.Printf("   IPv4 Address. . . . . . . . . . . : %s\n", adapter.IPv4Address)
	fmt.Printf("   Subnet Mask . . . . . . . . . . . : %s\n", adapter.SubnetMask)
	fmt.Printf("   Default Gateway . . . . . . . . . : %s\n", adapter.DefaultGateway)

	fmt.Println() // 각 어댑터 정보 후 개행
}

func main() {
	interfaceFlag := flag.String("interface", "", "Specify interface to display")
	jsonFlag := flag.Bool("json", false, "Output in JSON format")
	showAllFlag := flag.Bool("all", false, "Show detailed information")
	flag.Parse()

	if len(flag.Args()) > 0 {
		fmt.Fprintf(os.Stderr, "You entered the wrong command\n")
	}
	fmt.Println("Windows IP Configuration")
	fmt.Println()

	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting interfaces: %v\n", err)
		return
	}

	defaultGateways := getDefaultGateways()
	dnsServers := getDNSServers()

	var adapters []*AdapterInfo

	for _, iface := range interfaces {
		// Filter interface if -interface flag is provided
		if *interfaceFlag != "" && iface.Name != *interfaceFlag {
			continue
		}
		// 활성화된 네트워크 인터페이스
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		adapter := getAdapterInfo(iface, defaultGateways, *showAllFlag, dnsServers)
		if adapter != nil {
			adapters = append(adapters, adapter)
		}
	}

	if *jsonFlag {
		out, err := json.MarshalIndent(adapters, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating JSON output: %v\n", err)
			return
		}
		fmt.Println(string(out))
	} else {
		for _, adapter := range adapters {
			printAdapterInfo(adapter, *showAllFlag)
		}
	}
}
