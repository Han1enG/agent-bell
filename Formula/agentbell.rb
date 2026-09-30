class Agentbell < Formula
  desc "Local macOS notifications for Claude Code and Codex CLI"
  homepage "https://github.com/Han1enG/agent-bell"
  url "https://github.com/Han1enG/agent-bell/archive/refs/tags/v0.1.0.tar.gz"
  version "0.1.0"
  license "MIT"

  depends_on :macos
  depends_on "go" => :build

  def install
    system "go", "build", "-trimpath", "-ldflags", "-s -w -X main.version=0.1.0", "-o", "agentbell", "."
    bin.install "agentbell"

    app = libexec/"AgentBell.app/Contents"
    (app/"MacOS").mkpath
    (app/"Resources").mkpath
    system "clang", "-framework", "Cocoa", "native/AgentBellNotifier.m", "-o", app/"MacOS/AgentBellNotifier"
    cp "native/Info.plist", app/"Info.plist"
    cp "assets/agentbell-icon.png", app/"Resources/AgentBell.png"
  end

  test do
    assert_match "0.1.0", shell_output("#{bin}/agentbell version")
  end
end
