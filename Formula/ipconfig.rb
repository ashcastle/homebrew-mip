class Ipconfig < Formula
  desc "Windows-style ipconfig for macOS"
  homepage "https://github.com/ashcastle/homebrew-mip"
  url "https://github.com/ashcastle/homebrew-mip.git",
      tag:   "v1.2.0",
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

      Cache and DHCP actions require administrator privileges. Resolve this
      formula's executable before using sudo:
        sudo "$(command -v ipconfig)" /renew en0
    EOS
  end

  test do
    assert_match "Windows IP Configuration", shell_output("#{bin}/ipconfig")
    assert_match '"interface_name"', shell_output("#{bin}/ipconfig -json")
    help = shell_output("#{bin}/ipconfig help")
    assert_match "WINDOWS-COMPATIBLE OPTIONS", help
    assert_match "/renew [adapter]", help
    assert_match "Not applicable on macOS", shell_output("#{bin}/ipconfig /registerdns", 1)
  end
end
