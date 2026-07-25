class Ipconfig < Formula
  desc "Windows-style ipconfig for macOS"
  homepage "https://github.com/ashcastle/homebrew-mip"
  url "https://github.com/ashcastle/homebrew-mip.git",
      tag:   "v1.1.0",
      using: :git
  license "GPL-2.0"
  head "https://github.com/ashcastle/homebrew-mip.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/ipconfig"
  end

  def caveats
    <<~EOS
      macOS also includes an unrelated command at /usr/sbin/ipconfig.

      Verify this formula is active:
        command -v ipconfig

      If /usr/sbin/ipconfig appears first, initialize Homebrew in your shell:
        eval "$(brew shellenv)"
        exec "$SHELL" -l

      Apple's command remains available as /usr/sbin/ipconfig.
    EOS
  end

  test do
    assert_match "Windows IP Configuration", shell_output("#{bin}/ipconfig")
    assert_match '"interface_name"', shell_output("#{bin}/ipconfig -json")
    assert_match "WINDOWS-COMPATIBLE OPTIONS", shell_output("#{bin}/ipconfig -h")
  end
end
