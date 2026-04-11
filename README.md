[English](./README.md) | [한국어](./README.ko.md)

# ipconfig

`ipconfig` is a small macOS-focused CLI for printing the network details most people actually need: IPv4 address, subnet mask, default gateway, and optional interface metadata.

The project was previously exposed as `mip`. The user-facing command is now `ipconfig`.

## Why this exists

macOS ships `ifconfig`, but its output is noisy for routine checks. This tool keeps the default view short and adds structured output when you need to script against it.

## Features

- Concise text output for active interfaces with IPv4 configuration
- `-all` mode for MAC address, MTU, IPv6 addresses, and DNS servers
- `-interface <name>` to inspect a single interface
- `-json` for machine-readable output
- Explicit errors when a requested interface does not exist or has no usable IPv4 configuration

## Installation

### Homebrew

```bash
brew tap ashcastle/mip
brew install ashcastle/mip/ipconfig
```

### Go

```bash
go install github.com/ashcastle/mipconfig/cmd/ipconfig@latest
```

### Build from source

```bash
git clone https://github.com/ashcastle/homebrew-mip.git
cd homebrew-mip
go build -o ipconfig ./cmd/ipconfig
```

## Usage

```bash
ipconfig
ipconfig -all
ipconfig -interface en0
ipconfig -json
```

### Example text output

```text
ipconfig: Network Configuration

Ethernet adapter en0:
   IPv4 Address . . . . . . . . . . . : 192.168.0.10
   Subnet Mask . . . . . . . . . . . . : 255.255.255.0
   Default Gateway . . . . . . . . . . : 192.168.0.1
```

### Example JSON output

```json
[
  {
    "name": "en0",
    "type": "Ethernet adapter en0",
    "ipv4_address": "192.168.0.10",
    "subnet_mask": "255.255.255.0",
    "default_gateway": "192.168.0.1"
  }
]
```

## Important note for macOS users

macOS already includes `/usr/sbin/ipconfig`. A Homebrew-installed `ipconfig` will shadow the system command if your Homebrew `bin` directory appears earlier in `PATH`.

If you need the Apple-provided tool, call it explicitly:

```bash
/usr/sbin/ipconfig
```

## Development

Run tests with:

```bash
GOCACHE=/tmp/mipconfig-gocache go test ./...
```

Run the CLI locally with:

```bash
GOCACHE=/tmp/mipconfig-gocache go run ./cmd/ipconfig --help
```

## License

GPL-2.0
