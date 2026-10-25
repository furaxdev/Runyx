# Homebrew formula for Runyx (macOS/Linux via Homebrew).
# Not published to homebrew-core or a tap yet — this is the formula a
# tap repo (e.g. furaxdev/homebrew-runyx) would carry. Untested in this
# session (no macOS/brew available in this Linux sandbox) — see
# docs/MANUAL_TESTS.md.
class Runyx < Formula
  desc "100% local LLM runner — fast, Ollama/OpenAI-compatible, with a built-in UI"
  homepage "https://github.com/furaxdev/runyx"
  url "https://github.com/furaxdev/runyx/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "" # filled in by the release automation once a real tarball exists
  license "MIT"
  head "https://github.com/furaxdev/runyx.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/runyx"
  end

  service do
    run [opt_bin/"runyx", "serve"]
    keep_alive false
    log_path var/"log/runyx.log"
    error_log_path var/"log/runyx.log"
  end

  test do
    assert_match "runyx", shell_output("#{bin}/runyx version")
  end
end
