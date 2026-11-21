// Package colibri fetches and runs the colibri engine
// (https://github.com/JustVugg/colibri) as an alternative backend to
// llama-server for MoE ("mixture of experts") models: instead of
// requiring the whole model to fit in VRAM/RAM, colibri streams the
// routed experts a token doesn't need straight from disk, keeping only
// the hot ones resident. Runyx surfaces this as "GPU Saver"/"CPU
// Saver" (see docs/DECISIONS.md): opt-in, and only actually used for
// models it's likely to help.
//
// Unlike llama-server (a single self-contained binary), colibri ships
// as a small Python launcher (`coli`) plus several native engine
// binaries and Vulkan shaders, all extracted flat into one directory —
// see Ensure's doc comment for the exact, verified layout. Running it
// needs Python 3.10+ on the host; that's a real new dependency this
// feature adds, not hidden behind it.
package colibri

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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/furaxdev/runyx/internal/downloader"
	"github.com/furaxdev/runyx/internal/hwdetect"
)

// ReleasesAPI is the GitHub releases endpoint for colibri's latest
// release. Overridable in tests.
var ReleasesAPI = "https://api.github.com/repos/JustVugg/colibri/releases/latest"

// ghAsset mirrors the subset of GitHub's release-asset JSON needed
// here, matching internal/backend's own fetcher.
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

// LauncherName is the Python launcher script's filename, the same on
// every platform (it's invoked as `python3 coli ...`, never executed
// directly).
const LauncherName = "coli"

// Fetcher downloads and caches colibri releases under a directory, one
// subdirectory per (release tag, OS, arch).
type Fetcher struct {
	Dir    string
	Client *http.Client
}

// NewFetcher returns a Fetcher rooted at dir.
func NewFetcher(dir string) *Fetcher {
	return &Fetcher{Dir: dir, Client: http.DefaultClient}
}

// ErrUnsupportedPlatform is returned by PickAsset when colibri
// publishes no prebuilt release for info's OS/arch. As of v2.1.0 that
// means Linux/arm64 and macOS/amd64 (Intel) have no prebuilt asset at
// all -- real limitations of colibri's own release matrix, not
// something Runyx can work around short of building colibri from
// source itself, which is out of scope here.
var ErrUnsupportedPlatform = fmt.Errorf("colibri publishes no prebuilt release for this OS/architecture")

// PickAsset selects colibri's release asset for info's OS/arch,
// matching its actual (verified against a real downloaded v2.1.0
// release) naming: "colibri-<tag>-linux-x86_64.tar.gz",
// "colibri-<tag>-windows-x86_64.zip", "colibri-<tag>-macos-arm64.tar.gz".
// Unlike llama.cpp's release matrix, there is exactly one base asset
// per supported OS/arch -- CPU and Vulkan both ship in it already, so
// there's nothing to pick between there. See CUDAOverlayAsset for the
// separate Windows CUDA add-on.
func PickAsset(info hwdetect.Info, assets []ghAsset) (ghAsset, error) {
	osTag, archTag, ok := platformTags(info)
	if !ok {
		return ghAsset{}, ErrUnsupportedPlatform
	}
	ext := ".tar.gz"
	if info.OS == "windows" {
		ext = ".zip"
	}
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if !strings.HasSuffix(name, ext) {
			continue
		}
		if !strings.HasPrefix(name, "colibri-") {
			continue
		}
		// The Windows CUDA add-on also matches os/arch/ext (it's named
		// "colibri-<tag>-windows-x86_64-cuda.zip") -- it's an overlay on
		// top of the base archive, not a replacement for it, and must
		// never be picked as *the* asset here.
		if strings.Contains(name, "-cuda") {
			continue
		}
		if strings.Contains(name, osTag) && strings.Contains(name, archTag) {
			return a, nil
		}
	}
	return ghAsset{}, fmt.Errorf("no colibri release asset matched os=%s arch=%s", info.OS, info.Arch)
}

// CUDAOverlayAsset finds the Windows CUDA add-on archive, if present:
// "colibri-<tag>-windows-x86_64-cuda.zip" per the verified v2.1.0
// release. Unpacked over the base Windows archive (never standalone),
// it adds coli_cuda.dll for NVIDIA cards with compute capability 8.0+.
// There is no Linux CUDA prebuilt as of v2.1.0 -- Linux gets Vulkan
// (already in the base archive) regardless of an NVIDIA card being
// present; this is a real limitation of colibri's own release matrix.
func CUDAOverlayAsset(info hwdetect.Info, assets []ghAsset) (ghAsset, bool) {
	if info.OS != "windows" {
		return ghAsset{}, false
	}
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.HasPrefix(name, "colibri-") && strings.HasSuffix(name, "-cuda.zip") && strings.Contains(name, "x86_64") {
			return a, true
		}
	}
	return ghAsset{}, false
}

func platformTags(info hwdetect.Info) (osTag, archTag string, ok bool) {
	switch info.OS {
	case "linux":
		if info.Arch != "amd64" {
			return "", "", false
		}
		return "linux", "x86_64", true
	case "windows":
		if info.Arch != "amd64" {
			return "", "", false
		}
		return "windows", "x86_64", true
	case "darwin":
		if info.Arch != "arm64" {
			return "", "", false
		}
		return "macos", "arm64", true
	default:
		return "", "", false
	}
}

// ErrPythonNotFound is returned when no usable Python 3.10+ interpreter
// is on PATH. colibri's `coli` launcher is a Python script (its native
// engine binaries are fetched/extracted fine either way) -- this is a
// real, new system dependency GPU Saver adds, not hidden behind it.
var ErrPythonNotFound = fmt.Errorf("colibri needs Python 3.10 or newer on PATH, and none was found (see docs/USAGE.md)")

// FindPython returns a usable "python3" or "python" on PATH, preferring
// python3 (matching start-here.sh's own preference order).
func FindPython() (string, error) {
	for _, candidate := range []string{"python3", "python"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", ErrPythonNotFound
}

// Ensure returns the path to a usable `coli` launcher script for info,
// downloading and extracting colibri's release if not already cached.
// The archive extracts flat (verified against a real v2.1.0 download):
// `coli` (the Python launcher) sits next to several native engine
// binaries (colibri, deepseek_v4, inkling, kimi_k3, qwen36, ...) and a
// shaders/ directory colibri's Vulkan path loads from beside it --
// Runyx keeps the whole directory together rather than cherry-picking
// one binary the way backend.Fetcher does for llama-server, since
// `coli` locates its sibling engine binaries and shaders by its own
// script directory.
// CachedLauncher returns an already-extracted colibri launcher for
// info's os/arch, if one exists, without ever touching the network.
// Used at daemon startup: GPU Saver is opt-in, but even with it
// enabled, Runyx should never surprise a user with an unasked-for
// multi-tens-of-MB download merely from starting the daemon -- colibri
// is only actually fetched the first time `runyx gpu-saver import` (or
// Ensure, which Daemon.New deliberately does not call) is run.
func (f *Fetcher) CachedLauncher(info hwdetect.Info) (string, bool) {
	return f.anyCachedLauncher(info)
}

func (f *Fetcher) Ensure(ctx context.Context, info hwdetect.Info, onProgress downloader.ProgressFunc) (string, error) {
	release, err := f.fetchRelease(ctx, ReleasesAPI)
	if err != nil {
		if cached, ok := f.anyCachedLauncher(info); ok {
			return cached, nil
		}
		return "", fmt.Errorf("fetch latest colibri release: %w", err)
	}
	asset, err := PickAsset(info, release.Assets)
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(f.Dir, fmt.Sprintf("%s-%s-%s", release.TagName, info.OS, info.Arch))
	launcherPath := filepath.Join(destDir, LauncherName)
	if _, statErr := os.Stat(launcherPath); statErr == nil {
		return launcherPath, nil
	}

	if err := f.downloadAndExtract(ctx, asset, destDir, onProgress); err != nil {
		return "", err
	}

	if overlay, ok := CUDAOverlayAsset(info, release.Assets); ok && info.BestBackend() == "cuda" {
		// Best-effort: a failed CUDA overlay still leaves a fully usable
		// Vulkan/CPU install from the base archive above, so this
		// doesn't fail Ensure as a whole -- GPU Saver just runs without
		// CUDA acceleration on this machine, which is still correct,
		// only slower.
		_ = f.downloadAndExtract(ctx, overlay, destDir, nil)
	}

	if runtime.GOOS != "windows" {
		_ = filepath.Walk(destDir, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return nil
			}
			// The launcher and every native engine binary need +x; data
			// files (shaders, .py helper modules) don't, but chmod'ing
			// them too is harmless and simpler than telling them apart.
			return os.Chmod(path, 0o755)
		})
	}
	return launcherPath, nil
}

func (f *Fetcher) downloadAndExtract(ctx context.Context, asset ghAsset, destDir string, onProgress downloader.ProgressFunc) error {
	archivePath := filepath.Join(f.Dir, asset.Name)
	if err := downloader.Download(ctx, f.client(), asset.BrowserDownloadURL, archivePath, downloader.Options{
		Concurrency:  4,
		OnProgress:   onProgress,
		MinFreeBytes: asset.Size * 2,
	}); err != nil {
		return fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer os.Remove(archivePath)

	if asset.Digest != "" {
		if err := verifyDigest(archivePath, asset.Digest); err != nil {
			return fmt.Errorf("downloaded archive %s failed integrity check: %w", asset.Name, err)
		}
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	if strings.HasSuffix(strings.ToLower(asset.Name), ".zip") {
		return extractZip(archivePath, destDir)
	}
	return extractTarGz(archivePath, destDir)
}

func (f *Fetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return http.DefaultClient
}

// anyCachedLauncher looks for a previously extracted colibri install
// under f.Dir matching info's os/arch, regardless of release tag, used
// when GitHub's releases API is unreachable (rate-limited or offline)
// but a usable install already exists from a previous run -- the same
// fallback shape as backend.Fetcher's anyCachedBinary.
func (f *Fetcher) anyCachedLauncher(info hwdetect.Info) (string, bool) {
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
		launcherPath := filepath.Join(f.Dir, e.Name(), LauncherName)
		fi, statErr := os.Stat(launcherPath)
		if statErr != nil {
			continue
		}
		if mod := fi.ModTime().Unix(); best == "" || mod > bestMod {
			best, bestMod = launcherPath, mod
		}
	}
	return best, best != ""
}

func (f *Fetcher) fetchRelease(ctx context.Context, url string) (ghRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ghRelease{}, err
	}
	resp, err := f.client().Do(req)
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

func sha256Hex(path string) (string, error) {
	fh, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer fh.Close()
	h := sha256.New()
	if _, err := io.Copy(h, fh); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func verifyDigest(path, digest string) error {
	const prefix = "sha256:"
	if !strings.HasPrefix(digest, prefix) {
		return nil
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

// extractZip extracts every entry in archive into destDir, flat layout
// preserved (colibri's Windows release has no leading directory,
// verified against the real Linux tar.gz; the Windows zip is assumed
// to follow the same convention its own docs describe).
func extractZip(archive, destDir string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, zf := range r.File {
		dest := filepath.Join(destDir, zf.Name)
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		err = writeExtractedFile(dest, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(archive, destDir string) error {
	fh, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer fh.Close()
	gz, err := gzip.NewReader(fh)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		dest := filepath.Join(destDir, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, dest); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := writeExtractedFile(dest, tr); err != nil {
				return err
			}
		}
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
