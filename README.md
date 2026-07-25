[English](./README.md) | [한국어](./README.ko.md)

# ipconfig for macOS

`ipconfig` gives macOS a Windows-style view of its current TCP/IP
configuration. It uses macOS System Configuration data for service, DNS, DHCP,
and routing details instead of treating `/etc/resolv.conf` as the source of
truth.

```text
Windows IP Configuration

Wireless LAN adapter Wi-Fi:

   Connection-specific DNS Suffix . .  :
   IPv4 Address . . . . . . . . . . .  : 192.168.0.20
   Subnet Mask . . . . . . . . . . . . : 255.255.255.0
   Default Gateway . . . . . . . . . . : 192.168.0.1
```

## What it supports

- `ipconfig` for the Windows-style basic adapter view
- `ipconfig /all` for host, MAC, DHCP, DNS, IPv4/IPv6, lease, and disconnected
  adapter details, including DHCPv6 DUID/IAID when macOS exposes them
- `ipconfig '/?'`, `ipconfig -h`, or `ipconfig help` for help
- `/release`, `/renew`, `/release6`, and `/renew6` with optional adapter names
  or `*` wildcard matching
- `/displaydns` for available macOS Host cache entries
- `/flushdns` for the macOS Directory Service and `mDNSResponder` caches
- `-interface <pattern>` with case-insensitive `*` wildcard matching
- `-json` for stable machine-readable adapter data
- Physical, bridge, VPN, and disconnected macOS network services
- IPv6 link-local scope IDs and per-service DNS configuration

Windows-only concepts with no faithful macOS equivalent—`/registerdns`, DHCP
class IDs, and network compartments—return `Not applicable on macOS` instead
of pretending to succeed. `/all` uses the same wording for NetBIOS and WINS.

## Install

### Homebrew

```bash
brew tap ashcastle/mip
brew install ashcastle/mip/ipconfig
hash -r
```

Verify which command your shell will run:

```bash
command -v ipconfig
type -a ipconfig
```

The first result should normally be `/opt/homebrew/bin/ipconfig` on Apple
Silicon or `/usr/local/bin/ipconfig` on Intel. If `/usr/sbin/ipconfig` still
appears first, initialize Homebrew in your shell and start a new login shell:

```bash
eval "$(brew shellenv)"
exec "$SHELL" -l
```

### Go

```bash
go install github.com/ashcastle/homebrew-mip/cmd/ipconfig@latest
```

Ensure `$(go env GOPATH)/bin` precedes `/usr/sbin` in `PATH`.

### Build from source

```bash
git clone https://github.com/ashcastle/homebrew-mip.git
cd homebrew-mip
go build -o ipconfig ./cmd/ipconfig
./ipconfig /all
```

## Usage

```bash
ipconfig
ipconfig /all
ipconfig /renew en0
ipconfig /renew 'Wi*'
ipconfig /release6 en0
ipconfig /displaydns
ipconfig /flushdns
ipconfig -interface en0
ipconfig -interface 'en*'
ipconfig -json
ipconfig -h
ipconfig help
```

Quote `/?` and wildcard arguments in zsh because the shell otherwise treats
`?` and `*` as filename patterns:

```bash
ipconfig '/?'
ipconfig /\?
ipconfig -interface 'utun*'
```

JSON output is an array of normalized adapter objects and contains no banner or
diagnostics.

## Administrator actions

Cache and DHCP actions require administrator privileges on macOS. Resolve this
project's executable before invoking `sudo`, because the administrator's
`PATH` may otherwise select Apple's unrelated `/usr/sbin/ipconfig`:

```bash
sudo "$(command -v ipconfig)" /renew en0
sudo "$(command -v ipconfig)" /displaydns
sudo "$(command -v ipconfig)" /flushdns
```

The DHCP mappings use Apple's `/usr/sbin/ipconfig set` interface:

- `/release` → `NONE`
- `/renew` → `DHCP`
- `/release6` → `NONE-V6`
- `/renew6` → `AUTOMATIC-V6`

Apple documents this interface for test and debugging use. Actions without an
adapter target every eligible configured adapter, matching Windows behavior.
Use a specific adapter when you do not intend to affect every DHCP service.
`/flushdns` runs `dscacheutil -flushcache` and then sends `HUP` to
`mDNSResponder`; failure in either step is reported.

## macOS command-name conflict

macOS includes an unrelated low-level command at `/usr/sbin/ipconfig`.
Homebrew normally places its `bin` directory earlier in `PATH`, allowing this
project to provide the interactive `ipconfig` command.

The Apple command remains available explicitly:

```bash
/usr/sbin/ipconfig
```

## Architecture

The CLI and network collection code are separated:

- `internal/networkconfig` normalizes interfaces and macOS System Configuration
  records into a testable network model.
- Network actions use a separate injectable executor, so command paths,
  privileges, adapter eligibility, failures, and ordering are tested without
  releasing a real lease or flushing the live cache.
- `internal/ipconfig` parses Windows-compatible syntax and renders text or JSON.
- macOS collection uses `net.Interfaces`, `/usr/sbin/scutil`, and
  `/usr/sbin/networksetup`. Missing optional records do not fail the entire
  command.
- Non-macOS builds return a clear unsupported-platform error.

## Development

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/ipconfig
```

The automated test suite never releases an active lease, renews an interface,
or flushes the live DNS cache.

## License

GPL-2.0
