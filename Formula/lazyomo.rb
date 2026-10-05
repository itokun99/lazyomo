class Lazyomo < Formula
  desc "lazyomo — Go TUI editor for oh-my-openagent configs"
  homepage "https://github.com/itokun99/lazyomo"
  version "3.0.0"
  license "MIT"

  on_macos do
    on_intel do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-darwin-amd64"
      sha256 "ea3b563d6a9f634cbf91dc81f132cad4d5db929bb1eb5af2805f06652fb335d9"
    end

    on_arm do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-darwin-arm64"
      sha256 "8dd92329a594539e93beb300e2646741d066ad7b4a39eeeb92c0a9a477fa9c2f"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-linux-amd64"
      sha256 "1633fbdd97d7f12ac74f88b0731e2d7e31dec96b43f49f4b9857c917d0c58f3a"
    end

    on_arm do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-linux-arm64"
      sha256 "f9a7a60afe73b84267cc3b33cc1d2d25baf1fb7fd98a79e1e217cf074c59fd85"
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
