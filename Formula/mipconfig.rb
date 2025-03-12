class Mipconfig < Formula
    desc "간결한 네트워크 정보를 제공하는 ipconfig 도구 (macOS용)"
    homepage "https://github.com/ashcastle/homebrew-mip"
    url "https://github.com/ashcastle/homebrew-mip/archive/v1.0.0-beta.tar.gz"
    sha256 "53c7d288486f7982d8342099c2aa1e61a49c3edf3f2109c4fe93eb40a98efd65"
    license "GPL-2.0"
  
    depends_on "go" => :build
  
    def install
        system "go", "build", "-o", bin/"mip", "."
    end
  
    test do
      system "#{bin}/mipconfig", "-json"
    end
  end