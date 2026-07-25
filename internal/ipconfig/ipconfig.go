package ipconfig

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ashcastle/homebrew-mip/internal/networkconfig"
)

const CommandName = "ipconfig"

const collectionTimeout = 5 * time.Second

type dependencies struct {
	collect func(context.Context) (networkconfig.Configuration, error)
}

type options struct {
	showAll          bool
	jsonOutput       bool
	interfacePattern string
}

type unsupportedOptionError struct {
	option string
}

func (e unsupportedOptionError) Error() string {
	return fmt.Sprintf("%s is not supported on macOS yet; no network settings were changed", e.option)
}

func Run(stdout, stderr io.Writer, args []string) int {
	return run(stdout, stderr, args, dependencies{collect: networkconfig.Collect})
}

func run(stdout, stderr io.Writer, args []string, deps dependencies) int {
	normalized, err := normalizeArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", CommandName, err)
		return 2
	}

	parsed, help, err := parseOptions(stderr, normalized)
	if help {
		if _, writeErr := io.WriteString(stdout, usageText()); writeErr != nil {
			fmt.Fprintf(stderr, "%s: write output: %v\n", CommandName, writeErr)
			return 1
		}
		return 0
	}
	if err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), collectionTimeout)
	defer cancel()

	config, err := deps.collect(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", CommandName, err)
		return 1
	}

	if parsed.interfacePattern != "" {
		config.Adapters = filterAdapters(config.Adapters, parsed.interfacePattern)
		if len(config.Adapters) == 0 {
			fmt.Fprintf(stderr, "%s: no adapter matches %q\n", CommandName, parsed.interfacePattern)
			return 1
		}
	}

	if parsed.jsonOutput {
		if err := writeJSON(stdout, config.Adapters); err != nil {
			fmt.Fprintf(stderr, "%s: write output: %v\n", CommandName, err)
			return 1
		}
		return 0
	}

	if _, err := io.WriteString(stdout, renderText(config, parsed.showAll)); err != nil {
		fmt.Fprintf(stderr, "%s: write output: %v\n", CommandName, err)
		return 1
	}
	return 0
}

func normalizeArgs(args []string) ([]string, error) {
	normalized := make([]string, 0, len(args))
	for _, arg := range args {
		if !strings.HasPrefix(arg, "/") {
			normalized = append(normalized, arg)
			continue
		}

		switch strings.ToLower(arg) {
		case "/all":
			normalized = append(normalized, "-all")
		case "/?":
			normalized = append(normalized, "-help")
		case "/release", "/release6", "/renew", "/renew6", "/flushdns", "/registerdns",
			"/displaydns", "/showclassid", "/setclassid", "/allcompartments":
			return nil, unsupportedOptionError{option: arg}
		default:
			return nil, fmt.Errorf("unrecognized option %q; use /? for help", arg)
		}
	}
	return normalized, nil
}

func parseOptions(stderr io.Writer, args []string) (options, bool, error) {
	fs := flag.NewFlagSet(CommandName, flag.ContinueOnError)
	fs.SetOutput(stderr)

	var parsed options
	var help bool
	fs.BoolVar(&parsed.showAll, "all", false, "Display the full configuration for all adapters")
	fs.BoolVar(&parsed.jsonOutput, "json", false, "Output machine-readable JSON")
	fs.StringVar(&parsed.interfacePattern, "interface", "", "Display adapters matching a name or * wildcard")
	fs.BoolVar(&help, "h", false, "Display help")
	fs.BoolVar(&help, "help", false, "Display help")
	fs.Usage = func() {
		_, _ = io.WriteString(stderr, usageText())
	}

	if err := fs.Parse(args); err != nil {
		return options{}, false, err
	}
	if help {
		return options{}, true, nil
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "%s: unexpected arguments: %s\n", CommandName, strings.Join(fs.Args(), " "))
		_, _ = io.WriteString(stderr, usageText())
		return options{}, false, errors.New("unexpected arguments")
	}
	return parsed, false, nil
}

func usageText() string {
	return `
USAGE:
    ipconfig [/all] [/?]

WINDOWS-COMPATIBLE OPTIONS:
    /all                 Display the full TCP/IP configuration.
    /?                   Display this help.

MACOS EXTENSIONS:
    -interface <pattern> Display matching adapters; * is supported.
    -json                Output adapter data as JSON.

Unsupported Windows options: /release, /renew, /release6, /renew6,
/flushdns, /registerdns, /displaydns, /showclassid, /setclassid,
and /allcompartments. No network settings are changed for these options.
`
}

func filterAdapters(adapters []networkconfig.Adapter, pattern string) []networkconfig.Adapter {
	filtered := make([]networkconfig.Adapter, 0, len(adapters))
	for _, adapter := range adapters {
		if wildcardMatch(pattern, adapter.InterfaceName) ||
			wildcardMatch(pattern, adapter.Name) ||
			wildcardMatch(pattern, adapter.Description) {
			filtered = append(filtered, adapter)
		}
	}
	return filtered
}

func wildcardMatch(pattern, value string) bool {
	pattern = strings.ToLower(pattern)
	value = strings.ToLower(value)
	if pattern == "*" {
		return true
	}

	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == value
	}

	position := 0
	if parts[0] != "" {
		if !strings.HasPrefix(value, parts[0]) {
			return false
		}
		position = len(parts[0])
	}

	for index := 1; index < len(parts)-1; index++ {
		if parts[index] == "" {
			continue
		}
		offset := strings.Index(value[position:], parts[index])
		if offset < 0 {
			return false
		}
		position += offset + len(parts[index])
	}

	last := parts[len(parts)-1]
	if last == "" {
		return true
	}
	return strings.HasSuffix(value[position:], last)
}

func writeJSON(w io.Writer, adapters []networkconfig.Adapter) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(adapters)
}

func renderText(config networkconfig.Configuration, showAll bool) string {
	var output strings.Builder
	output.WriteString("\nWindows IP Configuration\n\n")

	if showAll {
		writeHostDetails(&output, config.Host)
	}

	if len(config.Adapters) == 0 {
		output.WriteString("   No network adapters were found.\n")
		return output.String()
	}

	for _, adapter := range config.Adapters {
		writeAdapter(&output, adapter, showAll)
	}
	return output.String()
}

func writeHostDetails(output *strings.Builder, host networkconfig.HostInfo) {
	writeField(output, "Host Name", valueOrBlank(host.HostName))
	writeField(output, "Primary Dns Suffix", host.PrimaryDNSSuffix)
	writeField(output, "Node Type", valueOrDefault(host.NodeType, "Hybrid"))
	writeField(output, "IP Routing Enabled", yesNo(host.IPRoutingEnabled))
	writeField(output, "WINS Proxy Enabled", yesNo(host.WINSProxyEnabled))
	output.WriteString("\n")
}

func writeAdapter(output *strings.Builder, adapter networkconfig.Adapter, showAll bool) {
	fmt.Fprintf(output, "%s adapter %s:\n\n", adapterHeading(adapter.Kind), adapter.Name)

	if !adapter.Connected {
		writeField(output, "Media State", "Media disconnected")
		writeField(output, "Connection-specific DNS Suffix", adapter.ConnectionSpecificSuffix)
		if showAll {
			writeAdapterMetadata(output, adapter)
		}
		output.WriteString("\n")
		return
	}

	writeField(output, "Connection-specific DNS Suffix", adapter.ConnectionSpecificSuffix)
	if showAll {
		writeAdapterMetadata(output, adapter)
	}

	for _, address := range adapter.IPv6Addresses {
		label := "IPv6 Address"
		if strings.HasPrefix(strings.ToLower(address), "fe80:") {
			label = "Link-local IPv6 Address"
		}
		if showAll {
			writeField(output, label, address+"(Preferred)")
		} else {
			writeField(output, label, address)
		}
	}

	for _, assignment := range adapter.IPv4Addresses {
		address := assignment.Address
		if showAll {
			address += "(Preferred)"
		}
		writeField(output, "IPv4 Address", address)
		writeField(output, "Subnet Mask", assignment.SubnetMask)
	}
	writeValues(output, "Default Gateway", adapter.DefaultGateways)

	if showAll {
		if adapter.DHCPEnabled {
			writeField(output, "DHCP Server", adapter.DHCPServer)
		}
		writeValues(output, "DNS Servers", adapter.DNSServers)
		if adapter.LeaseObtained != "" {
			writeField(output, "Lease Obtained", adapter.LeaseObtained)
		}
		if adapter.LeaseExpires != "" {
			writeField(output, "Lease Expires", adapter.LeaseExpires)
		}
	}
	output.WriteString("\n")
}

func writeAdapterMetadata(output *strings.Builder, adapter networkconfig.Adapter) {
	writeField(output, "Description", adapter.Description)
	writeField(output, "Physical Address", adapter.PhysicalAddress)
	writeField(output, "DHCP Enabled", yesNo(adapter.DHCPEnabled))
	writeField(output, "Autoconfiguration Enabled", yesNo(adapter.AutoconfigurationEnabled))
}

func writeValues(output *strings.Builder, label string, values []string) {
	if len(values) == 0 {
		writeField(output, label, "")
		return
	}
	writeField(output, label, values[0])
	for _, value := range values[1:] {
		fmt.Fprintf(output, "   %-36s: %s\n", "", value)
	}
}

func writeField(output *strings.Builder, label, value string) {
	fmt.Fprintf(output, "   %-36s: %s\n", dottedLabel(label), value)
}

func dottedLabel(label string) string {
	for len(label)+2 <= 35 {
		label += " ."
	}
	return label
}

func adapterHeading(kind networkconfig.AdapterKind) string {
	switch kind {
	case networkconfig.AdapterWireless:
		return "Wireless LAN"
	case networkconfig.AdapterEthernet, networkconfig.AdapterBridge:
		return "Ethernet"
	case networkconfig.AdapterTunnel:
		return "Tunnel"
	default:
		return "Unknown"
	}
}

func yesNo(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}

func valueOrBlank(value string) string {
	return value
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
