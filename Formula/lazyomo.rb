class Lazyomo < Formula
  desc "lazyomo — Go TUI editor for oh-my-openagent configs"
  homepage "https://github.com/itokun99/lazyomo"
  version "3.2.0"
  license "MIT"

  on_macos do
    on_intel do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-darwin-amd64"
      sha256 "c5edf97fa6baa01cf4ae8a26f445e9e7a3ab4b637e49fc736495363a921c57ee"
    end

    on_arm do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-darwin-arm64"
      sha256 "d5629c9b32252431751e1614c70c1426dda87751a1057cbdd692932b2bd4a4fb"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-linux-amd64"
      sha256 "19b71ab7f48846aa5c4e92c17124e9cc46adfea63effa4dc90957c8b35ad867f"
    end

    on_arm do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-linux-arm64"
      sha256 "b416fc9b2f58b0d2eb390bd317525d295eae7d850fa85d64252d3b2c493669b8"
    end
  end

  def install
    binary = Dir["lazyomo-*"].first
    raise "lazyomo binary not found in download" if binary.nil?
    bin.install binary => "lazyomo"
  end

  test do
    assert_match "lazyomo", shell_output("#{bin}/lazyomo --help")
  end
end
