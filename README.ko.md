[English](./README.md) | [한국어](./README.ko.md)

# ipconfig

`ipconfig`는 macOS에서 자주 확인하는 네트워크 정보만 간결하게 보여주는 작은 CLI입니다. 기본 출력은 IPv4 주소, 서브넷 마스크, 기본 게이트웨이에 집중하고, 필요할 때만 추가 정보를 보여줍니다.

이 프로젝트는 이전에 `mip` 명령으로 노출되어 있었고, 이제 사용자용 명령은 `ipconfig`로 통일되었습니다.

## 목적

macOS의 `ifconfig`는 일상적인 확인 용도로는 너무 장황합니다. 이 도구는 기본 출력은 짧게 유지하고, 스크립트 연동이 필요할 때는 구조화된 출력도 제공합니다.

## 주요 기능

- IPv4 설정이 있는 활성 인터페이스만 간결하게 출력
- `-all` 옵션으로 MAC 주소, MTU, IPv6 주소, DNS 서버 출력
- `-interface <name>`으로 특정 인터페이스만 조회
- `-json`으로 기계가 읽기 쉬운 출력 제공
- 존재하지 않는 인터페이스나 사용 가능한 IPv4 설정이 없는 인터페이스에 대해 명확한 오류 반환

## 설치

### Homebrew

```bash
brew tap ashcastle/mip
brew install ashcastle/mip/ipconfig
```

### Go

```bash
go install github.com/ashcastle/mipconfig/cmd/ipconfig@latest
```

### 소스에서 직접 빌드

```bash
git clone https://github.com/ashcastle/homebrew-mip.git
cd homebrew-mip
go build -o ipconfig ./cmd/ipconfig
```

## 사용법

```bash
ipconfig
ipconfig -all
ipconfig -interface en0
ipconfig -json
```

### 텍스트 출력 예시

```text
ipconfig: Network Configuration

Ethernet adapter en0:
   IPv4 Address . . . . . . . . . . . : 192.168.0.10
   Subnet Mask . . . . . . . . . . . . : 255.255.255.0
   Default Gateway . . . . . . . . . . : 192.168.0.1
```

### JSON 출력 예시

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

## macOS 사용자 주의사항

macOS에는 이미 `/usr/sbin/ipconfig`가 포함되어 있습니다. Homebrew로 설치한 `ipconfig`는 Homebrew의 `bin` 경로가 `PATH` 앞쪽에 있으면 시스템 명령을 가릴 수 있습니다.

Apple이 제공하는 기본 도구를 써야 한다면 다음처럼 명시적으로 호출하세요.

```bash
/usr/sbin/ipconfig
```

## 개발

테스트 실행:

```bash
GOCACHE=/tmp/mipconfig-gocache go test ./...
```

로컬 실행:

```bash
GOCACHE=/tmp/mipconfig-gocache go run ./cmd/ipconfig --help
```

## 라이선스

GPL-2.0
