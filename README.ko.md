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
  연결이 끊긴 어댑터까지 표시
- `ipconfig '/?'` 또는 `ipconfig -h`: 도움말
- `-interface <pattern>`: 대소문자를 구분하지 않는 `*` 와일드카드 검색
- `-json`: 안정적인 기계 판독용 어댑터 데이터
- 물리·브리지·VPN·연결 해제 macOS 네트워크 서비스 지원
- IPv6 link-local scope ID와 서비스별 DNS 구성 지원

`/release`, `/renew`, `/flushdns`처럼 네트워크 상태를 변경하는 Windows
옵션은 아직 의도적으로 구현하지 않았습니다. 이 옵션은 네트워크 정보를
수집하기 전에 실패하며, 아무 설정도 변경하지 않았음을 명시합니다.

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
ipconfig -interface en0
ipconfig -interface 'en*'
ipconfig -json
ipconfig -h
```

zsh는 `?`와 `*`를 파일 패턴으로 해석하므로 다음 인수는 따옴표로
감싸야 합니다.

```bash
ipconfig '/?'
ipconfig -interface 'utun*'
```

JSON 출력은 정규화된 어댑터 객체의 배열이며 배너나 진단 문구를 섞지
않습니다.

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

## 라이선스

GPL-2.0
