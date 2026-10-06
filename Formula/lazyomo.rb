class Lazyomo < Formula
  desc "lazyomo — Go TUI editor for oh-my-openagent configs"
  homepage "https://github.com/itokun99/lazyomo"
  version "3.1.0"
  license "MIT"

  on_macos do
    on_intel do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-darwin-amd64"
      sha256 "f3171644e253d92ce405c80d15d7a9525c8fecd53d549a4e788cb7b0b1c23443"
    end

    on_arm do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-darwin-arm64"
      sha256 "e831d8c2e9aa7a3fe40dc148b55b5b76a7720149af9a5af498b5c83cc6c1225a"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-linux-amd64"
      sha256 "9e5eb4c4a4532e87316000ef3579571ace58a443ef59e8f61d70ab4936453a66"
    end

    on_arm do
      url "https://github.com/itokun99/lazyomo/releases/download/v#{version}/lazyomo-linux-arm64"
      sha256 "8f19fb015fa80ad95ee6071d9f26a9cec726d2b7cb465c456a4ad9986481cf40"
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
