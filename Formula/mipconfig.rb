class Mipconfig < Formula
    desc "간결한 네트워크 정보를 제공하는 ipconfig 도구 (macOS용)"
    homepage "https://github.com/ashcastle/homebrew-mip"
    url "https://github.com/ashcastle/homebrew-mip/archive/v1.0.1-beta.tar.gz"
    sha256 "8e4453405388f7a912c34a6daf4499f2b0bdfec7af76bde44c926b83a90db2be"
    license "GPL-2.0"
  
    depends_on "go" => :build
  
    def install
        system "go", "build", "-o", bin/"mip", "."
    end
  
    test do
      system "#{bin}/mipconfig", "-json"
    end
  end