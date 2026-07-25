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
  adapter details
- `ipconfig '/?'` or `ipconfig -h` for help
- `-interface <pattern>` with case-insensitive `*` wildcard matching
- `-json` for stable machine-readable adapter data
- Physical, bridge, VPN, and disconnected macOS network services
- IPv6 link-local scope IDs and per-service DNS configuration

The state-changing Windows options such as `/release`, `/renew`, and
`/flushdns` are intentionally not implemented yet. They fail before collecting
network data and confirm that no settings were changed.

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
ipconfig -interface en0
ipconfig -interface 'en*'
ipconfig -json
ipconfig -h
```

Quote `/?` and wildcard arguments in zsh because the shell otherwise treats
`?` and `*` as filename patterns:

```bash
ipconfig '/?'
ipconfig -interface 'utun*'
```

JSON output is an array of normalized adapter objects and contains no banner or
diagnostics.

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

## License

GPL-2.0
