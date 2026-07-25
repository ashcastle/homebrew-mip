[English](./README.md) | [한국어](./README.ko.md)

# macOS용 ipconfig

`ipconfig`는 macOS의 현재 TCP/IP 구성을 Windows와 유사한 형식으로
보여줍니다. `/etc/resolv.conf`를 정답으로 간주하지 않고 macOS System
Configuration에서 네트워크 서비스, DNS, DHCP, 라우팅 정보를 수집합니다.

```text
Windows IP Configuration

Wireless LAN adapter Wi-Fi:

   Connection-specific DNS Suffix . .  :
   IPv4 Address . . . . . . . . . . .  : 192.168.0.20
   Subnet Mask . . . . . . . . . . . . : 255.255.255.0
   Default Gateway . . . . . . . . . . : 192.168.0.1
```

## 지원 범위

- `ipconfig`: Windows식 기본 어댑터 출력
- `ipconfig /all`: 호스트명, MAC, DHCP, DNS, IPv4/IPv6, 임대 시간,
  연결이 끊긴 어댑터와 macOS가 제공하는 DHCPv6 DUID/IAID까지 표시
- `ipconfig '/?'`, `ipconfig -h` 또는 `ipconfig help`: 도움말
- `/release`, `/renew`, `/release6`, `/renew6`: 선택적 어댑터 이름과 `*`
  와일드카드 지원
- `/displaydns`: macOS가 제공하는 Host 캐시 항목 표시
- `/flushdns`: macOS Directory Service와 `mDNSResponder` 캐시 비우기
- `-interface <pattern>`: 대소문자를 구분하지 않는 `*` 와일드카드 검색
- `-json`: 안정적인 기계 판독용 어댑터 데이터
- 물리·브리지·VPN·연결 해제 macOS 네트워크 서비스 지원
- IPv6 link-local scope ID와 서비스별 DNS 구성 지원

`/registerdns`, DHCP class ID, network compartments처럼 macOS에 대응
개념이 없는 기능은 성공한 것처럼 가장하지 않고 `Not applicable on
macOS`를 반환합니다. `/all`의 NetBIOS와 WINS도 같은 방식으로 표시합니다.

## 설치

### Homebrew

```bash
brew tap ashcastle/mip
brew install ashcastle/mip/ipconfig
hash -r
```

셸이 어떤 명령을 실행할지 확인합니다.

```bash
command -v ipconfig
type -a ipconfig
```

첫 결과는 Apple Silicon에서 보통 `/opt/homebrew/bin/ipconfig`, Intel
Mac에서는 `/usr/local/bin/ipconfig`여야 합니다. `/usr/sbin/ipconfig`가
먼저 나온다면 Homebrew 환경을 셸에 반영하고 로그인 셸을 다시 시작하세요.

```bash
eval "$(brew shellenv)"
exec "$SHELL" -l
```

### Go

```bash
go install github.com/ashcastle/homebrew-mip/cmd/ipconfig@latest
```

`$(go env GOPATH)/bin`이 `PATH`에서 `/usr/sbin`보다 앞에 있어야 합니다.

### 소스에서 빌드

```bash
git clone https://github.com/ashcastle/homebrew-mip.git
cd homebrew-mip
go build -o ipconfig ./cmd/ipconfig
./ipconfig /all
```

## 사용법

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

zsh는 `?`와 `*`를 파일 패턴으로 해석하므로 다음 인수는 따옴표로
감싸야 합니다.

```bash
ipconfig '/?'
ipconfig /\?
ipconfig -interface 'utun*'
```

JSON 출력은 정규화된 어댑터 객체의 배열이며 배너나 진단 문구를 섞지
않습니다.

## 관리자 권한이 필요한 동작

macOS에서는 캐시 및 DHCP 동작에 관리자 권한이 필요합니다. 관리자
`PATH`가 Apple의 `/usr/sbin/ipconfig`를 선택할 수 있으므로 이 프로젝트의
실행 경로를 먼저 확인한 뒤 `sudo`를 사용하세요.

```bash
sudo "$(command -v ipconfig)" /renew en0
sudo "$(command -v ipconfig)" /displaydns
sudo "$(command -v ipconfig)" /flushdns
```

DHCP 동작은 Apple `/usr/sbin/ipconfig set`에 다음처럼 대응됩니다.

- `/release` → `NONE`
- `/renew` → `DHCP`
- `/release6` → `NONE-V6`
- `/renew6` → `AUTOMATIC-V6`

Apple은 이 인터페이스를 테스트와 디버깅 용도로 설명합니다. 어댑터를
지정하지 않으면 Windows와 마찬가지로 조건에 맞는 모든 구성된 어댑터가
대상이 됩니다. 모든 DHCP 서비스를 건드릴 의도가 없다면 특정 어댑터를
지정하세요.
`/flushdns`는 `dscacheutil -flushcache` 실행 후 `mDNSResponder`에
`HUP` 신호를 보내며, 어느 단계든 실패하면 오류를 반환합니다.

## macOS 기본 명령과의 이름 충돌

macOS에는 이 프로젝트와 용도가 다른 `/usr/sbin/ipconfig`가 포함되어
있습니다. Homebrew의 `bin` 경로가 `PATH` 앞쪽에 있으면 이 프로젝트의
`ipconfig`가 평소 입력하는 명령으로 활성화됩니다.

Apple 기본 명령은 언제든 명시적으로 호출할 수 있습니다.

```bash
/usr/sbin/ipconfig
```

## 구조

- `internal/networkconfig`: 인터페이스와 macOS System Configuration
  레코드를 테스트 가능한 네트워크 모델로 정규화
- 별도의 주입 가능한 실행기가 명령 경로, 권한, 어댑터 적격성, 실패와
  실행 순서를 검증하므로 테스트 중 실제 lease나 캐시를 변경하지 않음
- `internal/ipconfig`: Windows 호환 인수 해석과 텍스트·JSON 출력 담당
- macOS 수집기는 `net.Interfaces`, `/usr/sbin/scutil`,
  `/usr/sbin/networksetup`을 사용하며 선택 정보가 없어도 전체 실행을
  실패시키지 않음
- macOS가 아닌 빌드에서는 지원하지 않는 플랫폼임을 명확히 반환

## 개발

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/ipconfig
```

자동 테스트는 활성 lease를 해제하거나 인터페이스를 갱신하거나 실제 DNS
캐시를 비우지 않습니다.

## 라이선스

GPL-2.0
