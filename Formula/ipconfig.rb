class Ipconfig < Formula
  desc "Concise macOS network configuration CLI"
  homepage "https://github.com/ashcastle/homebrew-mip"
  url "https://github.com/ashcastle/homebrew-mip.git", branch: "main", using: :git
  version "main"
  license "GPL-2.0"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/ipconfig"
  end

  def caveats
    <<~EOS
      This formula installs `ipconfig`, which can shadow macOS's built-in `/usr/sbin/ipconfig`
      if your Homebrew bin directory appears earlier in PATH.
    EOS
  end

  test do
    output = shell_output("#{bin}/ipconfig -json")
    assert_match "[", output
  end
end
