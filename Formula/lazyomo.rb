class Lazyomo < Formula
  desc "lazyomo — Go TUI editor for oh-my-openagent configs"
  homepage "https://github.com/itokun99/lazyomo"
  version "3.0.0"
  license "MIT"

  on_macos do
    on_intel do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-darwin-amd64"
      sha256 "2f2e0791027b50f264ef3ab4adf642391d7298e6a6468fbe43b18fcc884cc932"
    end

    on_arm do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-darwin-arm64"
      sha256 "6768f1d2bc4b02906829803eb97ef4f8c0c727046ac8e1fbf128a1932226badc"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-linux-amd64"
      sha256 "00af6b24097998aa51ed902f0d7c0ca9ab0fced5e4d62204b5ca2600632d03f8"
    end

    on_arm do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-linux-arm64"
      sha256 "8159bfa1e005e5ea40e07e651695147caf82d450403faf6ff1c33ad9c8efe620"
    end
  end

  def install
    binary_name = if OS.mac?
      Hardware::CPU.arm? ? "lazyomo-darwin-arm64" : "lazyomo-darwin-amd64"
    else
      Hardware::CPU.arm? ? "lazyomo-linux-arm64" : "lazyomo-linux-amd64"
    end

    bin.install binary_name => "lazyomo"
  end

  test do
    system "#{bin}/lazyomo", "--help"
  end
end
