# Tow - Lightweight deployment orchestrator
# by neurosam.AI — https://neurosam.ai
#
# This is a TEMPLATE. The actual formula with real SHA256 checksums
# lives in the neurosamAI/homebrew-tap repo and is auto-updated
# by the release workflow via repository_dispatch.
#
# To install: brew install neurosamAI/tap/tow

class Tow < Formula
  desc "Lightweight, agentless deployment orchestrator for bare-metal servers and VMs"
  homepage "https://tow-cli.neurosam.ai"
  license "MIT"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/neurosamAI/tow-cli/releases/download/v#{version}/tow-darwin-arm64"
      sha256 "UPDATE_ON_RELEASE"

      def install
        bin.install "tow-darwin-arm64" => "tow"
      end
    else
      url "https://github.com/neurosamAI/tow-cli/releases/download/v#{version}/tow-darwin-amd64"
      sha256 "UPDATE_ON_RELEASE"

      def install
        bin.install "tow-darwin-amd64" => "tow"
      end
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/neurosamAI/tow-cli/releases/download/v#{version}/tow-linux-arm64"
      sha256 "UPDATE_ON_RELEASE"

      def install
        bin.install "tow-linux-arm64" => "tow"
      end
    else
      url "https://github.com/neurosamAI/tow-cli/releases/download/v#{version}/tow-linux-amd64"
      sha256 "UPDATE_ON_RELEASE"

      def install
        bin.install "tow-linux-amd64" => "tow"
      end
    end
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tow --version")
  end
end
