package ipconfig

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ashcastle/homebrew-mip/internal/networkconfig"
)

const CommandName = "ipconfig"

const commandTimeout = 15 * time.Second

type dependencies struct {
	collect    func(context.Context) (networkconfig.Configuration, error)
	execute    func(context.Context, networkconfig.ActionRequest) (networkconfig.ActionResult, error)
	executable func() (string, error)
}

type options struct {
	showAll          bool
	jsonOutput       bool
	interfacePattern string
}

type invocation struct {
	options             options
	help                bool
	action              networkconfig.Action
	adapterPattern      string
	notApplicableOption string
}

func Run(stdout, stderr io.Writer, args []string) int {
	return run(stdout, stderr, args, dependencies{
		collect:    networkconfig.Collect,
		execute:    networkconfig.ExecuteAction,
		executable: os.Executable,
	})
}

func run(stdout, stderr io.Writer, args []string, deps dependencies) int {
	parsed, err := parseInvocation(stderr, args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", CommandName, err)
		return 2
	}

	if parsed.help {
		return writeResult(stdout, stderr, usageText())
	}
	if parsed.notApplicableOption != "" {
		if code := writeResult(stdout, stderr, renderNotApplicable(parsed.notApplicableOption)); code != 0 {
			return code
		}
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	if parsed.action != "" {
		return runAction(ctx, stdout, stderr, parsed, deps)
	}
	return runDisplay(ctx, stdout, stderr, parsed.options, deps)
}

func runDisplay(ctx context.Context, stdout, stderr io.Writer, parsed options, deps dependencies) int {
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
	return writeResult(stdout, stderr, renderText(config, parsed.showAll))
}

func runAction(ctx context.Context, stdout, stderr io.Writer, parsed invocation, deps dependencies) int {
	request := networkconfig.ActionRequest{Action: parsed.action}
	if actionNeedsAdapters(parsed.action) {
		config, err := deps.collect(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", CommandName, err)
			return 1
		}

		request.Adapters, err = selectActionAdapters(config.Adapters, parsed.action, parsed.adapterPattern)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", CommandName, err)
			return 1
		}
	}

	result, err := deps.execute(ctx, request)
	if err != nil {
		var privilegeError networkconfig.PrivilegeError
		if errors.As(err, &privilegeError) {
			fmt.Fprintf(stderr, "%s: %v\n", CommandName, privilegeError)
			if len(result.Adapters) > 0 {
				fmt.Fprintf(stderr, "Completed before failure: %s\n", strings.Join(result.Adapters, ", "))
			}
			if retry := privilegeRetryCommand(parsed, deps.executable); retry != "" {
				fmt.Fprintf(stderr, "Run: %s\n", retry)
			}
			return 1
		}
		fmt.Fprintf(stderr, "%s: %v\n", CommandName, err)
		if len(result.Adapters) > 0 {
			fmt.Fprintf(stderr, "Completed before failure: %s\n", strings.Join(result.Adapters, ", "))
		}
		return 1
	}

	var output string
	switch parsed.action {
	case networkconfig.ActionDisplayDNS:
		output = renderDNSCache(result)
	case networkconfig.ActionFlushDNS:
		output = "\nWindows IP Configuration\n\nSuccessfully flushed the DNS Resolver Cache.\n"
	default:
		output = renderAdapterAction(parsed.action, result.Adapters)
	}
	return writeResult(stdout, stderr, output)
}

func parseInvocation(stderr io.Writer, args []string) (invocation, error) {
	if len(args) == 1 && strings.EqualFold(args[0], "help") {
		return invocation{help: true}, nil
	}

	if len(args) > 0 && strings.HasPrefix(args[0], "/") {
		option := strings.ToLower(args[0])
		switch option {
		case "/release":
			return parseAdapterAction(networkconfig.ActionRelease4, args[1:])
		case "/renew":
			return parseAdapterAction(networkconfig.ActionRenew4, args[1:])
		case "/release6":
			return parseAdapterAction(networkconfig.ActionRelease6, args[1:])
		case "/renew6":
			return parseAdapterAction(networkconfig.ActionRenew6, args[1:])
		case "/displaydns":
			return parseStandaloneAction(networkconfig.ActionDisplayDNS, args[1:])
		case "/flushdns":
			return parseStandaloneAction(networkconfig.ActionFlushDNS, args[1:])
		case "/registerdns", "/showclassid", "/setclassid", "/allcompartments":
			return invocation{notApplicableOption: option}, nil
		}
	}

	normalized, err := normalizeDisplayArgs(args)
	if err != nil {
		return invocation{}, err
	}
	parsed, help, err := parseDisplayOptions(stderr, normalized)
	return invocation{options: parsed, help: help}, err
}

func parseAdapterAction(action networkconfig.Action, args []string) (invocation, error) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "/") {
			return invocation{}, fmt.Errorf("/%s cannot be combined with %q", action, arg)
		}
	}
	return invocation{
		action:         action,
		adapterPattern: strings.TrimSpace(strings.Join(args, " ")),
	}, nil
}

func parseStandaloneAction(action networkconfig.Action, args []string) (invocation, error) {
	if len(args) > 0 {
		return invocation{}, fmt.Errorf("/%s does not accept additional arguments", action)
	}
	return invocation{action: action}, nil
}

func normalizeDisplayArgs(args []string) ([]string, error) {
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
		default:
			return nil, fmt.Errorf("unrecognized option %q; use -h or help", arg)
		}
	}
	return normalized, nil
}

func parseDisplayOptions(stderr io.Writer, args []string) (options, bool, error) {
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
    ipconfig
    ipconfig /all
    ipconfig /release [adapter]
    ipconfig /renew [adapter]
    ipconfig /release6 [adapter]
    ipconfig /renew6 [adapter]
    ipconfig /displaydns
    ipconfig /flushdns
    ipconfig /?

WINDOWS-COMPATIBLE OPTIONS:
    /all                 Display the full TCP/IP configuration.
    /release [adapter]   Release DHCP-assigned IPv4 configuration.
    /renew [adapter]     Renew DHCP-assigned IPv4 configuration.
    /release6 [adapter]  Release automatic IPv6 configuration.
    /renew6 [adapter]    Renew automatic IPv6 configuration.
    /displaydns          Display available macOS Host cache entries.
    /flushdns            Flush the macOS DNS resolver caches.
    /?                   Display this help. In zsh, use /\? or -h.

MACOS EXTENSIONS:
    help                 Display help without shell escaping.
    -interface <pattern> Display matching adapters; * is supported.
    -json                Output adapter data as JSON.

Administrator privileges are required for cache and DHCP actions. The options
/registerdns, /showclassid, /setclassid, and /allcompartments have no faithful
macOS equivalent and report "Not applicable on macOS".
`
}

func selectActionAdapters(adapters []networkconfig.Adapter, action networkconfig.Action, pattern string) ([]networkconfig.Adapter, error) {
	matched := adapters
	if pattern != "" {
		matched = filterAdapters(adapters, pattern)
		if len(matched) == 0 {
			return nil, fmt.Errorf("no adapter matches %q", pattern)
		}
	}

	selected := make([]networkconfig.Adapter, 0, len(matched))
	for _, adapter := range matched {
		if adapterEligibleForAction(adapter, action) {
			selected = append(selected, adapter)
		}
	}
	if len(selected) == 0 {
		if pattern != "" {
			return nil, fmt.Errorf("adapter %q is not eligible for /%s", pattern, action)
		}
		return nil, fmt.Errorf("no adapters are eligible for /%s", action)
	}
	return selected, nil
}

func adapterEligibleForAction(adapter networkconfig.Adapter, action networkconfig.Action) bool {
	if adapter.InterfaceName == "" {
		return false
	}
	switch action {
	case networkconfig.ActionRelease4, networkconfig.ActionRenew4:
		return adapter.DHCPEnabled
	case networkconfig.ActionRelease6, networkconfig.ActionRenew6:
		return adapter.IPv6Automatic && adapter.Kind != networkconfig.AdapterTunnel &&
			adapter.Kind != networkconfig.AdapterVirtual
	default:
		return false
	}
}

func actionNeedsAdapters(action networkconfig.Action) bool {
	switch action {
	case networkconfig.ActionRelease4, networkconfig.ActionRenew4,
		networkconfig.ActionRelease6, networkconfig.ActionRenew6:
		return true
	default:
		return false
	}
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

func privilegeRetryCommand(parsed invocation, executable func() (string, error)) string {
	if executable == nil {
		return ""
	}
	path, err := executable()
	if err != nil || path == "" {
		return ""
	}
	command := "sudo " + shellQuote(path) + " /" + string(parsed.action)
	if parsed.adapterPattern != "" {
		command += " " + shellQuote(parsed.adapterPattern)
	}
	return command
}

func shellQuote(value string) string {
	if value != "" && strings.IndexFunc(value, func(character rune) bool {
		return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_./-", character)
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func writeJSON(w io.Writer, adapters []networkconfig.Adapter) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(adapters)
}

func writeResult(stdout, stderr io.Writer, output string) int {
	if _, err := io.WriteString(stdout, output); err != nil {
		fmt.Fprintf(stderr, "%s: write output: %v\n", CommandName, err)
		return 1
	}
	return 0
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
		writeAdapter(&output, adapter, config.Host.DHCPv6DUID, showAll)
	}
	return output.String()
}

func writeHostDetails(output *strings.Builder, host networkconfig.HostInfo) {
	writeField(output, "Host Name", host.HostName)
	writeField(output, "Primary Dns Suffix", host.PrimaryDNSSuffix)
	writeField(output, "Node Type", valueOrDefault(host.NodeType, "Hybrid"))
	writeField(output, "IP Routing Enabled", yesNo(host.IPRoutingEnabled))
	writeField(output, "WINS Proxy Enabled", "Not applicable on macOS")
	output.WriteString("\n")
}

func writeAdapter(output *strings.Builder, adapter networkconfig.Adapter, dhcpv6DUID string, showAll bool) {
	fmt.Fprintf(output, "%s adapter %s:\n\n", adapterHeading(adapter.Kind), adapter.Name)

	if !adapter.Connected {
		writeField(output, "Media State", "Media disconnected")
		writeField(output, "Connection-specific DNS Suffix", adapter.ConnectionSpecificSuffix)
		if showAll {
			writeAdapterMetadata(output, adapter, dhcpv6DUID)
		}
		output.WriteString("\n")
		return
	}

	writeField(output, "Connection-specific DNS Suffix", adapter.ConnectionSpecificSuffix)
	if showAll {
		writeAdapterMetadata(output, adapter, dhcpv6DUID)
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

func writeAdapterMetadata(output *strings.Builder, adapter networkconfig.Adapter, dhcpv6DUID string) {
	writeField(output, "Description", adapter.Description)
	writeField(output, "Physical Address", adapter.PhysicalAddress)
	writeField(output, "DHCP Enabled", yesNo(adapter.DHCPEnabled))
	writeField(output, "Autoconfiguration Enabled", yesNo(adapter.AutoconfigurationEnabled))
	if adapter.IPv6Automatic {
		writeField(output, "DHCPv6 IAID", valueOrDefault(adapter.DHCPv6IAID, "Not available on macOS"))
		writeField(output, "DHCPv6 Client DUID", valueOrDefault(dhcpv6DUID, "Not available on macOS"))
	}
	writeField(output, "NetBIOS over Tcpip", "Not applicable on macOS")
}

func renderDNSCache(result networkconfig.ActionResult) string {
	var output strings.Builder
	output.WriteString("\nWindows IP Configuration\n\n")
	output.WriteString("DNS Resolver Cache\n\n")

	if result.CacheRestricted {
		output.WriteString("   macOS did not make Host cache details available.\n")
		return output.String()
	}
	if len(result.DNSCache) == 0 {
		output.WriteString("   No DNS resolver cache entries were available.\n")
		return output.String()
	}

	for _, entry := range result.DNSCache {
		fmt.Fprintf(&output, "    %s\n", entry.Name)
		output.WriteString("    ----------------------------------------\n")
		writeField(&output, "Record Name", entry.Name)
		writeField(&output, "Record Type", entry.Type)
		writeField(&output, "Time To Live", "Not available on macOS")
		label := entry.Type + " (Host) Record"
		writeField(&output, label, entry.Address)
		output.WriteString("\n")
	}
	return output.String()
}

func renderAdapterAction(action networkconfig.Action, adapters []string) string {
	var verb string
	var protocol string
	switch action {
	case networkconfig.ActionRelease4:
		verb, protocol = "released", "IPv4"
	case networkconfig.ActionRenew4:
		verb, protocol = "renewed", "IPv4"
	case networkconfig.ActionRelease6:
		verb, protocol = "released", "IPv6"
	case networkconfig.ActionRenew6:
		verb, protocol = "renewed", "IPv6"
	}

	var output strings.Builder
	output.WriteString("\nWindows IP Configuration\n\n")
	fmt.Fprintf(&output, "Successfully %s the %s configuration for:\n", verb, protocol)
	for _, adapter := range adapters {
		fmt.Fprintf(&output, "   %s\n", adapter)
	}
	return output.String()
}

func renderNotApplicable(option string) string {
	reasons := map[string]string{
		"/registerdns":     "macOS does not provide Windows Dynamic DNS registration.",
		"/showclassid":     "macOS DHCP does not use Windows DHCP class IDs.",
		"/setclassid":      "macOS DHCP does not use Windows DHCP class IDs.",
		"/allcompartments": "macOS does not implement Windows network compartments.",
	}
	reason := reasons[option]
	return fmt.Sprintf("\nWindows IP Configuration\n\n%s is Not applicable on macOS.\n%s\n", option, reason)
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

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
