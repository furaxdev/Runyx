// Package backend manages the llama.cpp backend: fetching the right
// prebuilt `llama-server` for the current machine, and supervising one
// subprocess per loaded model.
package backend

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/furaxdev/runyx/internal/downloader"
	"github.com/furaxdev/runyx/internal/hwdetect"
)

// ReleasesAPI is the GitHub releases endpoint for upstream llama.cpp's
// latest release. Overridable in tests.
var ReleasesAPI = "https://api.github.com/repos/ggml-org/llama.cpp/releases/latest"

// releaseByTagAPI returns the GitHub API URL for one specific release
// tag. A separate var (rather than derived from ReleasesAPI) so tests can
// point it at a different test server independently of ReleasesAPI.
var releaseByTagAPI = func(tag string) string {
	return "https://api.github.com/repos/ggml-org/llama.cpp/releases/tags/" + tag
}

// nightlyTagAssetName is the asset llama.cpp's milestone releases (e.g.
// "v0.7.0") carry instead of prebuilt binaries: a one-line text file
// naming the continuous-build release tag (e.g. "b11669") that actually
// has the per-platform archives attached. "latest" resolves to whichever
// milestone tag is newest, which is frequently binary-less — this asset
// is upstream's own pointer to where the binaries really are.
const nightlyTagAssetName = "nightly-tag.txt"

// ghAsset mirrors the subset of GitHub's release-asset JSON we need.
// Digest, when GitHub's API reports one, is "sha256:<hex>" of the
// exact asset bytes -- verified against the download when present (not
// every asset on every repo has one; llama.cpp's don't appear to as of
// writing, but this costs nothing and activates automatically if that
// ever changes upstream).
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

// Fetcher downloads and caches llama-server builds under a backends
// directory, one subdirectory per (OS, arch, variant, release tag).
type Fetcher struct {
	Dir    string
	Client *http.Client
}

// NewFetcher returns a Fetcher rooted at dir.
func NewFetcher(dir string) *Fetcher {
	return &Fetcher{Dir: dir, Client: http.DefaultClient}
}

// serverBinaryName is "llama-server" or "llama-server.exe" depending on OS.
func serverBinaryName() string {
	if runtime.GOOS == "windows" {
		return "llama-server.exe"
	}
	return "llama-server"
}

// PickAsset selects the best release asset name fragment for the given
// hardware, matching upstream llama.cpp's own release-asset naming
// convention (documented in its release notes), e.g.
// "llama-<tag>-bin-ubuntu-x64.tar.gz", "llama-<tag>-bin-win-cuda-x64.zip",
// "llama-<tag>-bin-macos-arm64.tar.gz". Linux/macOS builds ship as
// .tar.gz, Windows builds as .zip.
func PickAsset(info hwdetect.Info, assets []ghAsset) (ghAsset, error) {
	osTag := map[string]string{"linux": "ubuntu", "windows": "win", "darwin": "macos"}[info.OS]
	archTag := map[string]string{"amd64": "x64", "arm64": "arm64"}[info.Arch]
	backend := info.BestBackend()

	var variantPriority []string
	switch backend {
	case "cuda":
		variantPriority = []string{"cuda", "vulkan", ""}
	case "rocm":
		variantPriority = []string{"hip", "rocm", "vulkan", ""}
	case "vulkan":
		variantPriority = []string{"vulkan", ""}
	case "metal":
		variantPriority = []string{""} // macOS builds bundle Metal support
	default:
		variantPriority = []string{""}
	}

	for _, variant := range variantPriority {
		for _, a := range assets {
			name := strings.ToLower(a.Name)
			if !strings.HasSuffix(name, ".zip") && !strings.HasSuffix(name, ".tar.gz") {
				continue
			}
			// "cudart-llama-...-cuda-...-x64.tar.gz" ships only the CUDA
			// runtime libraries alongside the real "llama-...-cuda-..."
			// build — matching it instead would silently fetch an
			// archive with no llama-server binary in it at all.
			if !strings.HasPrefix(name, "llama-") {
				continue
			}
			if !strings.Contains(name, osTag) || !strings.Contains(name, archTag) {
				continue
			}
			if variant == "" {
				// Prefer an asset with no extra GPU-vendor tag.
				if !containsAny(name, "cuda", "vulkan", "hip", "rocm") {
					return a, nil
				}
				continue
			}
			if strings.Contains(name, variant) {
				return a, nil
			}
		}
	}
	return ghAsset{}, fmt.Errorf("no matching release asset for os=%s arch=%s backend=%s", info.OS, info.Arch, backend)
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// Ensure returns the path to a usable llama-server binary for info,
// downloading and extracting it if not already cached.
func (f *Fetcher) Ensure(ctx context.Context, info hwdetect.Info, onProgress downloader.ProgressFunc) (string, error) {
	release, err := f.latestRelease(ctx)
	if err != nil {
		// GitHub's releases API is unauthenticated here and rate-limited
		// per source IP (60 req/hour) — easy to exhaust on a shared CI
		// runner pool, or offline. Resolving "latest" always requires
		// the network even when a perfectly usable build is already on
		// disk from a previous run, so before failing outright, fall
		// back to whatever release tag is already cached for this
		// os/arch rather than refusing to start at all.
		if cached, ok := f.anyCachedBinary(info); ok {
			return cached, nil
		}
		return "", fmt.Errorf("fetch latest llama.cpp release: %w", err)
	}
	asset, err := PickAsset(info, release.Assets)
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(f.Dir, fmt.Sprintf("%s-%s-%s", release.TagName, info.OS, info.Arch))
	binPath := filepath.Join(destDir, serverBinaryName())
	if _, err := os.Stat(binPath); err == nil {
		// Existing on disk only proves a file is there, not that it's
		// the one this Fetcher put there untouched -- verify it against
		// the hash recorded at extraction time before trusting it as a
		// subprocess binary. A mismatch (tampering, disk corruption, a
		// hand-placed substitute) is treated as "not actually cached":
		// fall through and re-download/re-extract rather than silently
		// execute something unverified.
		if verifyOrPinBinary(binPath) {
			return binPath, nil // already cached, and verified
		}
	}

	archivePath := filepath.Join(f.Dir, asset.Name)
	if err := downloader.Download(ctx, f.Client, asset.BrowserDownloadURL, archivePath, downloader.Options{
		Concurrency:  4,
		OnProgress:   onProgress,
		MinFreeBytes: asset.Size * 2,
	}); err != nil {
		return "", fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer os.Remove(archivePath)

	if asset.Digest != "" {
		if err := verifyDigest(archivePath, asset.Digest); err != nil {
			return "", fmt.Errorf("downloaded archive %s failed integrity check: %w", asset.Name, err)
		}
	}

	if err := extractServerBinary(archivePath, destDir); err != nil {
		return "", fmt.Errorf("extract %s: %w", asset.Name, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(binPath, 0o755); err != nil {
			return "", fmt.Errorf("chmod server binary: %w", err)
		}
	}
	if strings.TrimSpace(os.Getenv(SafeCPUKernelEnv)) != "" {
		pruneToBaselineCPUKernel(destDir, info)
	}
	// Record this freshly-extracted binary's hash so future runs (the
	// os.Stat fast path above, and anyCachedBinary's offline fallback)
	// can tell "still the file we wrote" from "something changed since".
	if err := pinBinary(binPath); err != nil {
		return "", fmt.Errorf("pin extracted binary: %w", err)
	}
	return binPath, nil
}

// sha256Hex returns the lowercase hex SHA-256 of a file's contents.
func sha256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// pinHashPath is the sidecar file recording binPath's expected SHA-256,
// written once right after extraction/chmod (i.e. before the binary is
// ever trusted or executed).
func pinHashPath(binPath string) string {
	return binPath + ".sha256"
}

// pinBinary records binPath's current SHA-256 to its sidecar file.
func pinBinary(binPath string) error {
	sum, err := sha256Hex(binPath)
	if err != nil {
		return err
	}
	return os.WriteFile(pinHashPath(binPath), []byte(sum), 0o644)
}

// verifyOrPinBinary reports whether binPath's current contents match
// its previously-pinned hash. A binary from before this check existed
// (no sidecar file yet) is pinned now rather than rejected outright --
// this is a hash recorded by Runyx itself, not an upstream-issued
// signature, so its guarantee is "unchanged since Runyx last wrote it",
// not "came from a trusted publisher"; a pre-existing install has no
// prior pin to compare against, and refusing to start over that
// distinction would be a regression for every existing user, not a
// security improvement. Once pinned, any later change — the actual
// threat this guards against — is caught.
func verifyOrPinBinary(binPath string) bool {
	sidecar := pinHashPath(binPath)
	recorded, err := os.ReadFile(sidecar)
	if err != nil {
		// No prior pin: trust-on-first-use, establish one now.
		return pinBinary(binPath) == nil
	}
	current, err := sha256Hex(binPath)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(recorded)) == current
}

// verifyDigest checks path's SHA-256 against a GitHub-style
// "sha256:<hex>" digest string.
func verifyDigest(path, digest string) error {
	const prefix = "sha256:"
	if !strings.HasPrefix(digest, prefix) {
		return nil // an unrecognized digest algorithm isn't a failure, just not checked
	}
	want := strings.TrimPrefix(digest, prefix)
	got, err := sha256Hex(path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("sha256 mismatch: got %s, want %s", got, want)
	}
	return nil
}

// SafeCPUKernelEnv, when set to a non-empty value, makes Ensure() delete
// every per-microarchitecture ggml-cpu kernel it just extracted except
// the baseline one, forcing llama.cpp's own runtime loader
// (ggml_backend_load_best, which scores each libggml-cpu-*.so by
// detected CPUID and keeps the highest scorer) to fall back to it
// instead of whatever it would otherwise have picked as "best".
//
// This exists because of a real, reproduced bug: the exact same model
// and llama.cpp build gave a coherent reply on an Intel Xeon runner and
// endless incoherent multilingual output — never emitting a stop token —
// on a GitHub-hosted AMD EPYC 7763 runner, across two different llama.cpp
// release tags (b11669 and b11836), so it isn't one bad build; something
// about the microarchitecture-specific kernel ggml selects for that CPU
// is wrong. llama.cpp has no supported env var to force a specific
// variant (checked its CPU-dispatch source directly), so this works
// around it the only way available: remove the higher-scoring variants
// so they can't be picked, trading their speed for the baseline kernel's
// (much better tested, lowest common denominator) correctness. Off by
// default — normal `runyx` keeps full performance; demo.yml opts in for
// CI determinism regardless of which CPU GitHub assigns the runner.
const SafeCPUKernelEnv = "RUNYX_LLAMA_CPP_SAFE_CPU_KERNEL"

// baselineCPUKernelName is the libggml-cpu-<name>.so llama.cpp ships as
// its no-special-instructions fallback for a given arch — present
// alongside the microarchitecture-specific variants only in the
// dynamic-dispatch Linux builds (the ones this prune applies to).
func baselineCPUKernelName(arch string) string {
	switch arch {
	case "amd64":
		return "libggml-cpu-x64.so"
	case "arm64":
		return "libggml-cpu-arm64.so"
	default:
		return ""
	}
}

// pruneToBaselineCPUKernel removes every libggml-cpu-<variant>.so file
// in dir except the baseline for info.Arch. A no-op (never an error) if
// the baseline itself isn't present — e.g. on macOS or Windows builds,
// which don't split ggml-cpu into multiple dynamically-dispatched
// variants the way the Linux build does, so there's nothing to prune.
func pruneToBaselineCPUKernel(dir string, info hwdetect.Info) {
	baseline := baselineCPUKernelName(info.Arch)
	if baseline == "" {
		return
	}
	if _, err := os.Stat(filepath.Join(dir, baseline)); err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if name == baseline {
			continue
		}
		if !strings.HasPrefix(name, "libggml-cpu-") || !strings.HasSuffix(name, ".so") {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}

// anyCachedBinary looks for a previously downloaded llama-server under
// f.Dir matching info's os/arch, regardless of release tag, picking the
// most recently extracted one if more than one is cached.
func (f *Fetcher) anyCachedBinary(info hwdetect.Info) (string, bool) {
	entries, err := os.ReadDir(f.Dir)
	if err != nil {
		return "", false
	}
	suffix := fmt.Sprintf("-%s-%s", info.OS, info.Arch)
	var best string
	var bestMod int64
	for _, e := range entries {
		if !e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		binPath := filepath.Join(f.Dir, e.Name(), serverBinaryName())
		fi, err := os.Stat(binPath)
		if err != nil || !verifyOrPinBinary(binPath) {
			continue
		}
		if mod := fi.ModTime().Unix(); best == "" || mod > bestMod {
			best, bestMod = binPath, mod
		}
	}
	return best, best != ""
}

// latestRelease resolves the release that actually has prebuilt binary
// assets. llama.cpp's GitHub "latest" release is often a source-only
// milestone tag carrying just a nightlyTagAssetName pointer; when that's
// the case, follow it to the continuous-build release underneath.
// PinnedReleaseTagEnv, when set, pins the exact llama.cpp build tag to
// fetch (e.g. "b11836") instead of always following whatever upstream
// currently reports as "latest". llama.cpp has no separate "stable"
// release channel — every published tag is a continuous-build nightly,
// and ggml-cpu's runtime CPU-microarchitecture dispatch means a given
// tag can behave correctly on one CPU and mis-generate (a real failure
// mode, not hypothetical — see docs/DECISIONS.md) on another. Pinning
// trades "always the newest build" for a specific, deliberately chosen
// tag, so a fix to the far more common case (something upstream) means
// updating this one tag rather than discovering a regression in
// production. Unset by default: normal `runyx` always follows latest.
const PinnedReleaseTagEnv = "RUNYX_LLAMA_CPP_TAG"

func (f *Fetcher) latestRelease(ctx context.Context) (ghRelease, error) {
	if tag := strings.TrimSpace(os.Getenv(PinnedReleaseTagEnv)); tag != "" {
		return f.fetchRelease(ctx, releaseByTagAPI(tag))
	}
	rel, err := f.fetchRelease(ctx, ReleasesAPI)
	if err != nil {
		return ghRelease{}, err
	}
	nightlyURL, ok := nightlyTagAssetURL(rel.Assets)
	if !ok {
		return rel, nil
	}
	tag, err := f.fetchNightlyTag(ctx, nightlyURL)
	if err != nil {
		return ghRelease{}, fmt.Errorf("resolve nightly build tag: %w", err)
	}
	return f.fetchRelease(ctx, releaseByTagAPI(tag))
}

func nightlyTagAssetURL(assets []ghAsset) (string, bool) {
	for _, a := range assets {
		if a.Name == nightlyTagAssetName {
			return a.BrowserDownloadURL, true
		}
	}
	return "", false
}

func (f *Fetcher) fetchNightlyTag(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	tag := strings.TrimSpace(string(b))
	if tag == "" {
		return "", fmt.Errorf("%s was empty", nightlyTagAssetName)
	}
	return tag, nil
}

func (f *Fetcher) fetchRelease(ctx context.Context, url string) (ghRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ghRelease{}, err
	}
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return ghRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return ghRelease{}, fmt.Errorf("status %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return ghRelease{}, err
	}
	return rel, nil
}

// extractServerBinary extracts just the llama-server binary (and any
// sibling .dll/.so/.dylib it needs) from archive into destDir. archive
// may be a .zip (Windows builds) or a .tar.gz (Linux/macOS builds).
func extractServerBinary(archive, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	if strings.HasSuffix(strings.ToLower(archive), ".tar.gz") {
		return extractServerBinaryFromTarGz(archive, destDir)
	}
	return extractServerBinaryFromZip(archive, destDir)
}

func wantedArchiveMembers() (wanted string, isWantedLib func(base string) bool) {
	wanted = serverBinaryName()
	isWantedLib = func(base string) bool {
		lower := strings.ToLower(base)
		if strings.HasSuffix(lower, ".dll") {
			return true
		}
		// Linux shared libraries commonly carry a version suffix after
		// ".so" (libllama-common.so.0, libggml.so.1.2.3, ...), and macOS
		// dylibs put the version before the extension
		// (libfoo.1.2.3.dylib) — a plain HasSuffix(".so"/".dylib") check
		// misses both, silently dropping a real runtime dependency
		// (this broke `runyx run` in CI: llama-server failed with
		// "error while loading shared libraries: libllama-common.so.0:
		// cannot open shared object file").
		return strings.Contains(lower, ".so") || strings.Contains(lower, ".dylib")
	}
	return
}

func extractServerBinaryFromZip(archive, destDir string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()

	wanted, isWantedLib := wantedArchiveMembers()
	found := false
	for _, zf := range r.File {
		base := filepath.Base(zf.Name)
		isServer := base == wanted
		if !isServer && !isWantedLib(base) {
			continue
		}
		if isServer {
			found = true
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		if zf.Mode()&os.ModeSymlink != 0 {
			// A zip stores a symlink's target path as the entry's
			// "content" rather than real file bytes.
			target, readErr := io.ReadAll(rc)
			rc.Close()
			if readErr != nil {
				return readErr
			}
			err = os.Symlink(string(target), filepath.Join(destDir, base))
		} else {
			err = writeExtractedFile(filepath.Join(destDir, base), rc)
			rc.Close()
		}
		if err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("archive %s did not contain %s", archive, wanted)
	}
	return nil
}

func extractServerBinaryFromTarGz(archive, destDir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	wanted, isWantedLib := wantedArchiveMembers()
	found := false
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Upstream ships each versioned shared library as a chain of
		// symlinks down to the real file (libfoo.so -> libfoo.so.0 ->
		// libfoo.so.1.2.3), and the binary dlopens it by one of the
		// symlinked names, not the real one — skipping TypeSymlink
		// entries (as if only TypeReg mattered) silently dropped the
		// exact filename llama-server actually needed, even though the
		// real file landed on disk under its own name. This is the
		// fix for a real CI failure: llama-server downloaded and
		// "extracted" fine, then failed at the dynamic linker with
		// "libllama-common.so.0: cannot open shared object file" —
		// that name was always a symlink, never a regular file.
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeSymlink {
			continue
		}
		base := filepath.Base(hdr.Name)
		isServer := base == wanted
		if !isServer && !isWantedLib(base) {
			continue
		}
		if isServer {
			found = true
		}
		if hdr.Typeflag == tar.TypeSymlink {
			if err := os.Symlink(hdr.Linkname, filepath.Join(destDir, base)); err != nil {
				return err
			}
			continue
		}
		if err := writeExtractedFile(filepath.Join(destDir, base), tr); err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("archive %s did not contain %s", archive, wanted)
	}
	return nil
}

func writeExtractedFile(dest string, src io.Reader) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}
