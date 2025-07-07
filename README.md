# MIP (macOS IP Configuration)

[![License: GPL v2](https://img.shields.io/badge/License-GPL%20v2-blue.svg)](https://www.gnu.org/licenses/old-licenses/gpl-2.0.en.html)
[![Go Version](https://img.shields.io/badge/Go-1.24.0-00ADD8.svg)](https://golang.org/)
[![Platform](https://img.shields.io/badge/Platform-macOS-lightgrey.svg)](https://www.apple.com/macos/)

**MIP**는 macOS용 경량 네트워크 구성 도구로, Windows의 `ipconfig` 명령어와 유사한 기능을 제공합니다. 간결하고 직관적인 인터페이스로 네트워크 어댑터 정보를 빠르게 확인할 수 있습니다.

## 🚀 주요 기능

- **간결한 출력**: 핵심 네트워크 정보만 표시
- **상세 모드**: `-all` 플래그로 IPv6, MAC 주소, MTU, DNS 서버 등 상세 정보 제공
- **인터페이스 선택**: `-interface` 플래그로 특정 네트워크 인터페이스만 조회
- **JSON 출력**: `-json` 플래그로 구조화된 데이터 형식 지원
- **강력한 오류 처리**: 네트워크 상태 변화에 대한 안정적인 대응
- **크로스 플랫폼 호환**: macOS 전용 최적화

## 📦 설치

### Homebrew를 통한 설치 (권장)

```bash
# Homebrew tap 추가
brew tap ashcastle/homebrew-mip

# mip 설치
brew install mipconfig
```

### 소스에서 직접 빌드

```bash
# 저장소 클론
git clone https://github.com/ashcastle/homebrew-mip.git
cd homebrew-mip

# Go 빌드
go build -o mip .

# 실행 파일을 PATH에 추가
sudo mv mip /usr/local/bin/
```

## 🔧 사용법

### 기본 사용

```bash
# 기본 네트워크 정보 출력
mip
```

### 상세 정보 출력

```bash
# 모든 상세 정보 포함
mip -all
```

### 특정 인터페이스 조회

```bash
# en0 인터페이스만 조회
mip -interface en0
```

### JSON 형식 출력

```bash
# JSON 형식으로 출력
mip -json

# 상세 정보를 JSON으로 출력
mip -all -json
```

## 📋 출력 예시

### 기본 출력

```
mip: Network Configuration

Ethernet adapter en0:
   IPv4 Address. . . . . . . . . . . : 192.168.1.100
   Subnet Mask . . . . . . . . . . . : 255.255.255.0
   Default Gateway . . . . . . . . . : 192.168.1.1

Wi-Fi adapter en1:
   IPv4 Address. . . . . . . . . . . : 192.168.1.101
   Subnet Mask . . . . . . . . . . . : 255.255.255.0
   Default Gateway . . . . . . . . . : 192.168.1.1
```

### 상세 출력 (-all)

```
mip: Network Configuration

Ethernet adapter en0:
   Link-local IPv6 Address . . . . . : fe80::1234:5678:9abc:def0
   Physical Address. . . . . . . . . : aa:bb:cc:dd:ee:ff
   MTU . . . . . . . . . . . . . . . : 1500
   DNS Servers . . . . . . . . . . . : 8.8.8.8, 8.8.4.4
   IPv4 Address. . . . . . . . . . . : 192.168.1.100
   Subnet Mask . . . . . . . . . . . : 255.255.255.0
   Default Gateway . . . . . . . . . : 192.168.1.1
```

### JSON 출력

```json
[
  {
    "name": "en0",
    "type": "Ethernet adapter en0",
    "ipv4_address": "192.168.1.100",
    "subnet_mask": "255.255.255.0",
    "default_gateway": "192.168.1.1",
    "ipv6_addresses": ["fe80::1234:5678:9abc:def0"],
    "physical_address": "aa:bb:cc:dd:ee:ff",
    "mtu": 1500,
    "dns_servers": ["8.8.8.8", "8.8.4.4"]
  }
]
```

## 🛠 기술 스택

- **언어**: Go 1.24.0
- **표준 라이브러리**: 
  - `net`: 네트워크 인터페이스 관리
  - `encoding/json`: JSON 출력 지원
  - `flag`: 명령줄 인수 처리
  - `os/exec`: 시스템 명령 실행
- **시스템 의존성**: 
  - `netstat`: 라우팅 테이블 조회
  - `/etc/resolv.conf`: DNS 서버 정보

## 📁 프로젝트 구조

```
homebrew-mip/
├── main.go              # 메인 애플리케이션 로직
├── go.mod               # Go 모듈 정의
├── Formula/
│   └── mipconfig.rb     # Homebrew Formula
├── LICENSE              # GPL-2.0 라이선스
└── README.md            # 프로젝트 문서
```

## 🔍 핵심 구현 사항

### 네트워크 인터페이스 감지
- Go의 `net.Interfaces()` 함수를 사용하여 시스템의 모든 네트워크 인터페이스 탐지
- 활성화된 인터페이스만 필터링 (`net.FlagUp` 확인)
- 인터페이스 타입 자동 분류 (Ethernet, Wi-Fi, Loopback)

### 라우팅 정보 수집
- `netstat -rn` 명령어를 통한 기본 게이트웨이 정보 수집
- 다양한 라우팅 테이블 형식에 대한 강력한 파싱
- 인터페이스별 게이트웨이 매핑

### DNS 서버 탐지
- `/etc/resolv.conf` 파일 파싱
- `nameserver` 항목 추출 및 검증

### 어댑터 타입 분류
```go
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
```

## 🎯 사용 사례

- **네트워크 문제 진단**: 빠른 IP 구성 확인
- **시스템 관리**: 자동화 스크립트에서 JSON 출력 활용
- **개발 환경 설정**: 로컬 네트워크 구성 확인
- **교육 목적**: 네트워크 개념 학습 도구

## 🚀 성능 최적화

- **메모리 효율성**: 구조체 기반 데이터 관리로 메모리 사용량 최소화
- **실행 속도**: 시스템 명령어 최소 호출로 빠른 응답 시간
- **에러 핸들링**: 네트워크 상태 변화에 대한 안정적인 대응

## 🔮 향후 계획

- [ ] **IPv6 지원 강화**: 더 상세한 IPv6 주소 정보 제공
- [ ] **네트워크 통계**: 인터페이스별 트래픽 통계 추가
- [ ] **설정 파일 지원**: 사용자 정의 출력 형식 설정
- [ ] **실시간 모니터링**: 네트워크 상태 변화 감지 모드
- [ ] **다국어 지원**: 한국어, 영어 등 다국어 출력 지원

## 🤝 기여하기

프로젝트에 기여를 환영합니다! 다음과 같은 방법으로 참여할 수 있습니다:

1. **이슈 리포트**: 버그나 개선 사항을 GitHub Issues에 등록
2. **풀 리퀘스트**: 코드 개선이나 새로운 기능 추가
3. **문서 개선**: README나 코드 주석 개선
4. **테스트**: 다양한 macOS 환경에서의 테스트

### 개발 환경 설정

```bash
# 저장소 포크 및 클론
git clone https://github.com/YOUR_USERNAME/homebrew-mip.git
cd homebrew-mip

# 의존성 확인
go mod tidy

# 빌드 및 테스트
go build -o mip .
./mip -json
```

## 📄 라이선스

이 프로젝트는 [GNU General Public License v2.0](LICENSE) 하에 배포됩니다.

## 👨‍💻 개발자

**ashcastle** - [GitHub](https://github.com/ashcastle)

---

⭐ 이 프로젝트가 유용하다면 스타를 눌러주세요!

📧 문의사항이나 제안사항이 있으시면 GitHub Issues를 통해 연락해 주세요.