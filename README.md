# mip (ipconfig in MacOS)

## 개요
이 프로젝트는 macOS에서 ifconfig 명령어가 출력하는 방대한 불필요한 정보 대신, Windows의 ipconfig와 유사한 방식으로 필요한 네트워크 정보만을 간결하게 확인할 수 있도록 제작되었습니다.  
ifconfig의 너무 많은 정보가 오히려 불편하여, 오직 필수적인 네트워크 정보(IPv4 주소, 서브넷 마스크, 기본 게이트웨이 등)만을 깔끔하게 보여줍니다.

## 주요 기능
- **간결한 출력:** 불필요한 정보를 제거하고, 필요한 네트워크 정보만 보기 쉽게 표시합니다.
- **상세 정보 옵션:** `-all` 옵션을 사용하면 물리적 주소(Physical Address), MTU, IPv6 주소, DNS 서버 정보 등 추가적인 상세 정보를 함께 출력합니다.
- **특정 인터페이스 선택:** `-interface` 옵션을 통해 특정 네트워크 인터페이스의 정보만 선택적으로 확인할 수 있습니다.
- **JSON 형식 출력:** `-json` 옵션을 사용하여 출력 결과를 JSON 형식으로 받아, 다른 도구와의 연동이나 스크립트 처리에 용이합니다.
- **강화된 에러 처리:** 네트워크 인터페이스 정보, DNS 서버 정보, 기본 게이트웨이 파싱 등에서 오류 발생 시 표준 에러 스트림(stderr)을 통해 명확한 에러 메시지를 제공합니다.

## 설치 방법
**Homebrew**를 사용하여 간편하게 설치할 수 있습니다. (brew 배포 관련 내용은 사장님께서 관리하시는 부분입니다.)

```bash
brew tap ashcastle/homebrew-mip
brew install ashcastle/mip
```
## 사용 방법

설치 후 터미널에서 `mip -help` 명령어를 실행하면 기본 네트워크 정보를 확인할 수 있습니다.

### 예시
	•	기본 실행

`mip`


	•	상세 정보 포함 실행

`mip -all`


	•	특정 인터페이스 정보 출력

`mip -interface en0`


	•	JSON 형식 출력

`mip -json`



## 기여 방법

> 이 프로젝트는 GNU GPL 라이선스 하에 운영되며, 기여자 여러분의 참여를 적극 환영합니다.
> 버그 리포트, 기능 개선, 문서 업데이트 등 다양한 기여를 통해 함께 발전시켜 나갈 수 있습니다.

## 라이선스

> 이 프로젝트는 GNU GPL 라이선스 하에 배포됩니다.
