# Homebrew formula for bolt (client CLI).
# After publishing a release, update sha256 lines from SHA256SUMS on the release page.
# Install without a tap:
#   brew install --formula https://raw.githubusercontent.com/g-savitha/flick/main/packaging/homebrew/Formula/bolt.rb
class Bolt < Formula
  desc "Fast terminal P2P chat and file transfer"
  homepage "https://github.com/g-savitha/flick"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/g-savitha/flick/releases/download/v0.1.0/bolt_0.1.0_darwin_arm64.tar.gz"
      sha256 "REPLACE_AFTER_FIRST_RELEASE"
    end
    on_intel do
      url "https://github.com/g-savitha/flick/releases/download/v0.1.0/bolt_0.1.0_darwin_amd64.tar.gz"
      sha256 "REPLACE_AFTER_FIRST_RELEASE"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/g-savitha/flick/releases/download/v0.1.0/bolt_0.1.0_linux_arm64.tar.gz"
      sha256 "REPLACE_AFTER_FIRST_RELEASE"
    end
    on_intel do
      url "https://github.com/g-savitha/flick/releases/download/v0.1.0/bolt_0.1.0_linux_amd64.tar.gz"
      sha256 "REPLACE_AFTER_FIRST_RELEASE"
    end
  end

  def install
    bin.install "bolt"
  end

  test do
    assert_match "0.1.0", shell_output("#{bin}/bolt version 2>&1")
  end
end
