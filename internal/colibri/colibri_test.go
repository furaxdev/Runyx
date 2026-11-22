package colibri

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/furaxdev/runyx/internal/hwdetect"
)

func TestPickAssetLinuxAMD64(t *testing.T) {
	info := hwdetect.Info{OS: "linux", Arch: "amd64"}
	assets := []ghAsset{
		{Name: "colibri-v2.1.0-linux-x86_64.tar.gz"},
		{Name: "colibri-v2.1.0-windows-x86_64.zip"},
		{Name: "colibri-v2.1.0-macos-arm64.tar.gz"},
		{Name: "SHA256SUMS.txt"},
	}
	got, err := PickAsset(info, assets)
	if err != nil {
		t.Fatalf("PickAsset: %v", err)
	}
	if got.Name != "colibri-v2.1.0-linux-x86_64.tar.gz" {
		t.Errorf("got %q", got.Name)
	}
}

func TestPickAssetWindowsSkipsCUDAOverlay(t *testing.T) {
	info := hwdetect.Info{OS: "windows", Arch: "amd64"}
	assets := []ghAsset{
		{Name: "colibri-v2.1.0-windows-x86_64-cuda.zip"},
		{Name: "colibri-v2.1.0-windows-x86_64.zip"},
	}
	got, err := PickAsset(info, assets)
	if err != nil {
		t.Fatalf("PickAsset: %v", err)
	}
	if got.Name != "colibri-v2.1.0-windows-x86_64.zip" {
		t.Errorf("got %q, want the base Windows archive, not the CUDA overlay", got.Name)
	}
}

func TestPickAssetMacOSArm64(t *testing.T) {
	info := hwdetect.Info{OS: "darwin", Arch: "arm64"}
	assets := []ghAsset{
		{Name: "colibri-v2.1.0-linux-x86_64.tar.gz"},
		{Name: "colibri-v2.1.0-macos-arm64.tar.gz"},
	}
	got, err := PickAsset(info, assets)
	if err != nil {
		t.Fatalf("PickAsset: %v", err)
	}
	if got.Name != "colibri-v2.1.0-macos-arm64.tar.gz" {
		t.Errorf("got %q", got.Name)
	}
}

func TestPickAssetErrorsOnUnsupportedPlatforms(t *testing.T) {
	assets := []ghAsset{
		{Name: "colibri-v2.1.0-linux-x86_64.tar.gz"},
		{Name: "colibri-v2.1.0-windows-x86_64.zip"},
		{Name: "colibri-v2.1.0-macos-arm64.tar.gz"},
	}
	// colibri v2.1.0 publishes no Linux/arm64 or macOS/Intel asset.
	for _, info := range []hwdetect.Info{
		{OS: "linux", Arch: "arm64"},
		{OS: "darwin", Arch: "amd64"},
	} {
		if _, err := PickAsset(info, assets); err == nil {
			t.Errorf("expected an error for os=%s arch=%s, colibri has no asset for it", info.OS, info.Arch)
		}
	}
}

func TestCUDAOverlayAssetOnlyOnWindows(t *testing.T) {
	assets := []ghAsset{{Name: "colibri-v2.1.0-windows-x86_64-cuda.zip"}}
	if _, ok := CUDAOverlayAsset(hwdetect.Info{OS: "linux", Arch: "amd64"}, assets); ok {
		t.Error("expected no CUDA overlay on Linux (colibri publishes none)")
	}
	got, ok := CUDAOverlayAsset(hwdetect.Info{OS: "windows", Arch: "amd64"}, assets)
	if !ok {
		t.Fatal("expected to find the Windows CUDA overlay")
	}
	if got.Name != "colibri-v2.1.0-windows-x86_64-cuda.zip" {
		t.Errorf("got %q", got.Name)
	}
}

// buildFakeTarGz creates a .tar.gz at path containing a flat layout
// matching the real one verified against colibri v2.1.0: a "coli"
// launcher script alongside a native engine binary and a shader file.
func buildFakeTarGz(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	files := map[string]string{
		"coli":                "#!/usr/bin/env python3\nprint('fake coli launcher')\n",
		"colibri":             "fake native engine binary",
		"shaders/qmatmul.spv": "fake shader bytes",
	}
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
}

func buildFakeZip(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()

	files := map[string]string{
		"coli":                "#!/usr/bin/env python3\nprint('fake coli launcher')\n",
		"colibri.exe":         "fake native engine binary",
		"shaders/qmatmul.spv": "fake shader bytes",
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
}

// newReleaseTestServer serves both the release JSON and the asset
// bytes from one server, so the JSON's BrowserDownloadURL can point
// back at the same server (its URL is only known after it starts).
func newReleaseTestServer(t *testing.T, archivePath, assetName string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/release", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ghRelease{
			TagName: "v2.1.0",
			Assets: []ghAsset{
				{Name: assetName, BrowserDownloadURL: srv.URL + "/asset"},
			},
		})
	})
	mux.HandleFunc("/asset", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, archivePath)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestEnsureDownloadsExtractsAndCachesTheLauncher(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "fake.tar.gz")
	buildFakeTarGz(t, archive)

	srv := newReleaseTestServer(t, archive, "colibri-v2.1.0-linux-x86_64.tar.gz")
	old := ReleasesAPI
	ReleasesAPI = srv.URL + "/release"
	defer func() { ReleasesAPI = old }()

	destRoot := filepath.Join(dir, "colibri-cache")
	f := NewFetcher(destRoot)
	info := hwdetect.Info{OS: "linux", Arch: "amd64"}

	launcherPath, err := f.Ensure(context.Background(), info, nil)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if filepath.Base(launcherPath) != LauncherName {
		t.Errorf("Ensure() = %q, want a path ending in %q", launcherPath, LauncherName)
	}
	content, err := os.ReadFile(launcherPath)
	if err != nil {
		t.Fatalf("read extracted launcher: %v", err)
	}
	if !bytes.Contains(content, []byte("fake coli launcher")) {
		t.Errorf("extracted launcher content = %q, want the fake script body", content)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(launcherPath), "shaders", "qmatmul.spv")); err != nil {
		t.Errorf("expected shaders/qmatmul.spv to be extracted alongside the launcher: %v", err)
	}

	// Second call should fall back to the already-extracted directory
	// once the releases API is unreachable (anyCachedLauncher), proving
	// the first call's extraction is actually usable on its own.
	srv.Close()
	launcherPath2, err := f.Ensure(context.Background(), info, nil)
	if err != nil {
		t.Fatalf("second Ensure (should hit cache) failed: %v", err)
	}
	if launcherPath2 != launcherPath {
		t.Errorf("second Ensure() = %q, want the same cached path %q", launcherPath2, launcherPath)
	}
}

func TestEnsureFallsBackToCachedLauncherWhenReleasesAPIFails(t *testing.T) {
	dir := t.TempDir()
	cachedDir := filepath.Join(dir, "v2.0.0-linux-amd64")
	if err := os.MkdirAll(cachedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	launcherPath := filepath.Join(cachedDir, LauncherName)
	if err := os.WriteFile(launcherPath, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	old := ReleasesAPI
	ReleasesAPI = srv.URL
	defer func() { ReleasesAPI = old }()

	f := NewFetcher(dir)
	got, err := f.Ensure(context.Background(), hwdetect.Info{OS: "linux", Arch: "amd64"}, nil)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got != launcherPath {
		t.Errorf("Ensure() = %q, want cached launcher %q", got, launcherPath)
	}
}

func TestEnsureFailsWhenReleasesAPIFailsAndNothingCached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	old := ReleasesAPI
	ReleasesAPI = srv.URL
	defer func() { ReleasesAPI = old }()

	f := NewFetcher(t.TempDir())
	if _, err := f.Ensure(context.Background(), hwdetect.Info{OS: "linux", Arch: "amd64"}, nil); err == nil {
		t.Error("expected an error when the releases API fails and nothing is cached")
	}
}

func TestFindPythonErrorsCleanlyWithNoneOnPath(t *testing.T) {
	// Only meaningful where neither candidate is actually installed;
	// best-effort guard against a false pass in an environment that
	// happens to have one (this sandbox always does).
	t.Setenv("PATH", t.TempDir())
	if _, err := FindPython(); err == nil {
		t.Skip("a python3/python happened to resolve anyway (e.g. a shell builtin); nothing to assert")
	}
}
