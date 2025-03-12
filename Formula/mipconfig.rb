class Mipconfig < Formula
    desc "간결한 네트워크 정보를 제공하는 ipconfig 도구 (macOS용)"
    homepage "https://github.com/ashcastle/mipconfig"
    url "https://github.com/ashcastle/mipconfig/archive/v1.0.0-beta.tar.gz"
    sha256 "f6888fbc79ad7e0e1212dec1185c434d6fe3702efc12ae178c370cd44e12a422"
    license "GPL-3.0"
  
    depends_on "go" => :build
  
    def install
      system "go", "build", "-o", bin/"mipconfig", "."
    end
  
    test do
      system "#{bin}/mipconfig", "-json"
    end
  end