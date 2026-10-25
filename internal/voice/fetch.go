// Package voice manages the local voice pipeline: whisper.cpp for
// speech-to-text and Piper for text-to-speech, both run as supervised
// subprocesses (same philosophy as internal/backend for llama.cpp — no
// cgo, independently updatable binaries). Implements FEATURES.md items
// 17 (audio attachment transcription) and 25 (full voice pipeline).
package voice

import (
	"archive/zip"
	"context"
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

// ghAsset/ghRelease mirror the GitHub releases API shape, duplicated
// from internal/backend rather than shared, to keep each fetcher
// independently simple (the two differ slightly in matching rules).
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

// Fetcher downloads and caches a prebuilt binary from a GitHub repo's
// latest release into dir, picking the asset whose name matches the
// current OS/arch.
type Fetcher struct {
	Repo          string // "owner/name", e.g. "ggerganov/whisper.cpp"
	ReleasesAPI   string // override for tests; defaults to the real GitHub API for Repo
	Dir           string
	Client        *http.Client
	BinaryName    string   // e.g. "whisper-cli" / "whisper-cli.exe"
	AssetContains []string // candidate substrings to prefer, in priority order (e.g. os/arch tags)
}

func (f *Fetcher) releasesURL() string {
	if f.ReleasesAPI != "" {
		return f.ReleasesAPI
	}
	return fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", f.Repo)
}

func (f *Fetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return http.DefaultClient
}

// PickAsset returns the first asset whose lowercased name contains all
// of the given substrings, trying each candidate substring set in order
// until one matches, so the caller can list fallbacks from most to least
// specific.
func PickAsset(assets []ghAsset, candidateSubstrSets ...[]string) (ghAsset, error) {
	for _, set := range candidateSubstrSets {
		for _, a := range assets {
			name := strings.ToLower(a.Name)
			if !strings.HasSuffix(name, ".zip") {
				continue
			}
			allMatch := true
			for _, want := range set {
				if !strings.Contains(name, want) {
					allMatch = false
					break
				}
			}
			if allMatch {
				return a, nil
			}
		}
	}
	return ghAsset{}, fmt.Errorf("no release asset matched any candidate set %v", candidateSubstrSets)
}

// OSArchCandidates returns default substring sets for info, most to
// least specific, matching common release-asset naming conventions
// (e.g. "win-x64", "macos-arm64", "ubuntu-x64" / "linux-x64").
func OSArchCandidates(info hwdetect.Info) [][]string {
	archTag := map[string]string{"amd64": "x64", "arm64": "arm64"}[info.Arch]
	switch info.OS {
	case "windows":
		return [][]string{{"win", archTag}}
	case "darwin":
		return [][]string{{"macos", archTag}, {"darwin", archTag}}
	default:
		return [][]string{{"ubuntu", archTag}, {"linux", archTag}}
	}
}

// Ensure returns the cached binary path, fetching and extracting it from
// the latest GitHub release if not already cached.
func (f *Fetcher) Ensure(ctx context.Context, candidateSets [][]string, onProgress downloader.ProgressFunc) (string, error) {
	release, err := f.latestRelease(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch latest release for %s: %w", f.Repo, err)
	}
	asset, err := PickAsset(release.Assets, candidateSets...)
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(f.Dir, release.TagName)
	binName := f.BinaryName
	if runtime.GOOS == "windows" && !strings.HasSuffix(binName, ".exe") {
		binName += ".exe"
	}
	binPath := filepath.Join(destDir, binName)
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	zipPath := filepath.Join(f.Dir, asset.Name)
	if err := downloader.Download(ctx, f.client(), asset.BrowserDownloadURL, zipPath, downloader.Options{
		Concurrency:  4,
		OnProgress:   onProgress,
		MinFreeBytes: asset.Size * 2,
	}); err != nil {
		return "", fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer os.Remove(zipPath)

	if err := extractBinary(zipPath, destDir, binName); err != nil {
		return "", fmt.Errorf("extract %s: %w", asset.Name, err)
	}
	if runtime.GOOS != "windows" {
		os.Chmod(binPath, 0o755)
	}
	return binPath, nil
}

func (f *Fetcher) latestRelease(ctx context.Context) (ghRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.releasesURL(), nil)
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

func extractBinary(archive, destDir, wanted string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	found := false
	for _, zf := range r.File {
		base := filepath.Base(zf.Name)
		isWanted := base == wanted
		isLib := strings.HasSuffix(base, ".dll") || strings.HasSuffix(base, ".so") || strings.HasSuffix(base, ".dylib")
		if !isWanted && !isLib {
			continue
		}
		if isWanted {
			found = true
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(filepath.Join(destDir, base))
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	if !found {
		return fmt.Errorf("archive did not contain %s", wanted)
	}
	return nil
}
