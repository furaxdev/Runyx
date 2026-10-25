package backend

import (
	"archive/tar"
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

func TestPickAssetPrefersCUDAOnNvidia(t *testing.T) {
	info := hwdetect.Info{OS: "linux", Arch: "amd64", GPUs: []hwdetect.GPU{{Backend: "cuda"}}}
	assets := []ghAsset{
		{Name: "llama-b1-bin-ubuntu-x64.tar.gz"},
		{Name: "llama-b1-bin-ubuntu-cuda-x64.tar.gz"},
		{Name: "llama-b1-bin-win-x64.zip"},
	}
	got, err := PickAsset(info, assets)
	if err != nil {
		t.Fatalf("PickAsset: %v", err)
	}
	if got.Name != "llama-b1-bin-ubuntu-cuda-x64.tar.gz" {
		t.Errorf("got %q, want the cuda build", got.Name)
	}
}

func TestPickAssetFallsBackToPlainCPUBuild(t *testing.T) {
	info := hwdetect.Info{OS: "linux", Arch: "amd64"}
	assets := []ghAsset{
		{Name: "llama-b1-bin-ubuntu-cuda-x64.tar.gz"},
		{Name: "llama-b1-bin-ubuntu-x64.tar.gz"},
		{Name: "llama-b1-bin-macos-arm64.tar.gz"},
	}
	got, err := PickAsset(info, assets)
	if err != nil {
		t.Fatalf("PickAsset: %v", err)
	}
	if got.Name != "llama-b1-bin-ubuntu-x64.tar.gz" {
		t.Errorf("got %q, want the plain CPU build", got.Name)
	}
}

func TestPickAssetErrorsWhenNoMatch(t *testing.T) {
	info := hwdetect.Info{OS: "windows", Arch: "arm64"}
	assets := []ghAsset{{Name: "llama-b1-bin-ubuntu-x64.tar.gz"}}
	if _, err := PickAsset(info, assets); err == nil {
		t.Error("expected error when no asset matches os/arch")
	}
}

// TestPickAssetSkipsCudartRuntimeOnlyPackage guards against a real
// upstream naming collision: llama.cpp's CUDA releases ship both
// "llama-<tag>-bin-ubuntu-cuda-<ver>-x64.tar.gz" (the actual
// llama-server build) and "cudart-llama-<tag>-bin-ubuntu-cuda-<ver>-x64.tar.gz"
// (just the CUDA runtime libraries, no llama-server binary at all) —
// both contain "ubuntu", "x64" and "cuda", so without excluding the
// cudart-prefixed one explicitly, alphabetical/JSON ordering could pick
// an archive with no server binary in it.
func TestPickAssetSkipsCudartRuntimeOnlyPackage(t *testing.T) {
	info := hwdetect.Info{OS: "linux", Arch: "amd64", GPUs: []hwdetect.GPU{{Backend: "cuda"}}}
	assets := []ghAsset{
		{Name: "cudart-llama-b1-bin-ubuntu-cuda-12.8-x64.tar.gz"},
		{Name: "llama-b1-bin-ubuntu-cuda-12.8-x64.tar.gz"},
	}
	got, err := PickAsset(info, assets)
	if err != nil {
		t.Fatalf("PickAsset: %v", err)
	}
	if got.Name != "llama-b1-bin-ubuntu-cuda-12.8-x64.tar.gz" {
		t.Errorf("got %q, want the real build, not the cudart-only package", got.Name)
	}
}

// TestEnsureFallsBackToCachedBinaryWhenReleasesAPIFails guards against a
// real CI failure: GitHub's unauthenticated releases API is rate-limited
// (60 req/hour per source IP), easily exhausted on a shared runner pool.
// Ensure() always resolves "latest" before it can even look at what's on
// disk, so a 403 there used to hard-fail `runyx run` even with a fully
// usable llama-server already cached from a previous run.
func TestEnsureFallsBackToCachedBinaryWhenReleasesAPIFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	old := ReleasesAPI
	ReleasesAPI = srv.URL
	defer func() { ReleasesAPI = old }()

	dir := t.TempDir()
	info := hwdetect.Info{OS: "linux", Arch: "amd64"}
	cachedDir := filepath.Join(dir, "b1234-linux-amd64")
	if err := os.MkdirAll(cachedDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	binPath := filepath.Join(cachedDir, serverBinaryName())
	if err := os.WriteFile(binPath, []byte("fake binary"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	f := NewFetcher(dir)
	got, err := f.Ensure(context.Background(), info, nil)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got != binPath {
		t.Errorf("Ensure() = %q, want cached binary %q", got, binPath)
	}
}

// TestEnsureFailsWhenReleasesAPIFailsAndNothingCached makes sure the
// fallback above doesn't paper over the case where there really is
// nothing usable on disk: Ensure() must still report the original error.
func TestEnsureFailsWhenReleasesAPIFailsAndNothingCached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	old := ReleasesAPI
	ReleasesAPI = srv.URL
	defer func() { ReleasesAPI = old }()

	f := NewFetcher(t.TempDir())
	info := hwdetect.Info{OS: "linux", Arch: "amd64"}
	if _, err := f.Ensure(context.Background(), info, nil); err == nil {
		t.Error("expected an error when the releases API fails and nothing is cached")
	}
}

// TestLatestReleaseFollowsNightlyTagPointer guards against a real CI
// failure: llama.cpp's "latest" GitHub release is sometimes a
// source-only milestone tag with no prebuilt binaries, carrying only a
// nightly-tag.txt asset that names the continuous-build release (e.g.
// "b11669") which actually has them. latestRelease() must follow that
// pointer instead of returning the binary-less milestone release as-is.
func TestLatestReleaseFollowsNightlyTagPointer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/nightly-tag.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("b11669\n"))
	})
	mux.HandleFunc("/tags/b11669", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghRelease{
			TagName: "b11669",
			Assets:  []ghAsset{{Name: "llama-b11669-bin-ubuntu-x64.tar.gz"}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghRelease{
			TagName: "v0.7.0",
			Assets: []ghAsset{
				{Name: "v0.7.0.zip"},
				{Name: "v0.7.0.tar.gz"},
				{Name: nightlyTagAssetName, BrowserDownloadURL: srv.URL + "/nightly-tag.txt"},
			},
		})
	})

	oldReleasesAPI, oldByTag := ReleasesAPI, releaseByTagAPI
	ReleasesAPI = srv.URL + "/latest"
	releaseByTagAPI = func(tag string) string { return srv.URL + "/tags/" + tag }
	defer func() { ReleasesAPI, releaseByTagAPI = oldReleasesAPI, oldByTag }()

	f := NewFetcher(t.TempDir())
	rel, err := f.latestRelease(context.Background())
	if err != nil {
		t.Fatalf("latestRelease: %v", err)
	}
	if rel.TagName != "b11669" {
		t.Errorf("TagName = %q, want the nightly tag, not the milestone release", rel.TagName)
	}
	if len(rel.Assets) != 1 || rel.Assets[0].Name != "llama-b11669-bin-ubuntu-x64.tar.gz" {
		t.Errorf("Assets = %+v, want the nightly release's binary assets", rel.Assets)
	}
}

// TestLatestReleaseHonorsPin guards the pin mechanism: with
// RUNYX_LLAMA_CPP_TAG set, latestRelease() must fetch that exact tag
// directly, bypassing both "latest" and the nightly-tag indirection
// entirely (a pinned tag already names a real build with binaries, so
// there's nothing to follow).
func TestLatestReleaseHonorsPin(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		t.Error("latestRelease should not call the /latest endpoint when a tag is pinned")
	})
	mux.HandleFunc("/tags/b11836", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghRelease{
			TagName: "b11836",
			Assets:  []ghAsset{{Name: "llama-b11836-bin-ubuntu-x64.tar.gz"}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	oldReleasesAPI, oldByTag := ReleasesAPI, releaseByTagAPI
	ReleasesAPI = srv.URL + "/latest"
	releaseByTagAPI = func(tag string) string { return srv.URL + "/tags/" + tag }
	defer func() { ReleasesAPI, releaseByTagAPI = oldReleasesAPI, oldByTag }()

	t.Setenv(PinnedReleaseTagEnv, "b11836")

	f := NewFetcher(t.TempDir())
	rel, err := f.latestRelease(context.Background())
	if err != nil {
		t.Fatalf("latestRelease: %v", err)
	}
	if rel.TagName != "b11836" {
		t.Errorf("TagName = %q, want the pinned tag", rel.TagName)
	}
}

// TestExtractServerBinaryFromTarGz guards the .tar.gz extraction path
// (used on Linux and macOS, where llama.cpp ships tar.gz rather than
// zip) end-to-end: build a real tar.gz containing a server binary, a
// versioned shared library it depends on, and an unrelated file, extract
// it, and check exactly the wanted files land. This is a real regression:
// a plain HasSuffix(".so") check missed "libllama-common.so.0" (the
// version suffix comes after ".so", not before it), so llama-server
// downloaded and extracted fine but then failed at startup with "error
// while loading shared libraries: libllama-common.so.0: cannot open
// shared object file" — exactly what CI hit.
func TestExtractServerBinaryFromTarGz(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "llama-b1-bin-ubuntu-x64.tar.gz")
	writeTestTarGz(t, archive, map[string]string{
		"build/bin/" + serverBinaryName(): "fake server binary",
		"build/bin/libllama-common.so.0":  "fake versioned shared lib",
		"build/bin/libggml-base.so":       "fake unversioned shared lib",
		"build/bin/README.md":             "not wanted",
	})

	destDir := filepath.Join(dir, "out")
	if err := extractServerBinary(archive, destDir); err != nil {
		t.Fatalf("extractServerBinary: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(destDir, serverBinaryName()))
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(got) != "fake server binary" {
		t.Errorf("extracted binary content = %q", got)
	}
	if _, err := os.ReadFile(filepath.Join(destDir, "libllama-common.so.0")); err != nil {
		t.Errorf("expected the versioned shared library to be extracted: %v", err)
	}
	if _, err := os.ReadFile(filepath.Join(destDir, "libggml-base.so")); err != nil {
		t.Errorf("expected the unversioned shared library to be extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "README.md")); !os.IsNotExist(err) {
		t.Error("expected README.md to NOT be extracted")
	}
}

// TestExtractServerBinaryFollowsSymlinkedLibraries guards the real CI
// failure that TestExtractServerBinaryFromTarGz's plain-regular-file
// setup didn't catch: upstream llama.cpp ships each versioned shared
// library as a symlink chain (libllama-common.so -> libllama-common.so.0
// -> libllama-common.so.0.7.0, the last being the only regular file),
// and llama-server dlopens it by the *symlinked* name
// ("libllama-common.so.0"), not the real one. Treating only
// tar.TypeReg entries as extractable silently dropped every symlink,
// so the real file landed on disk under its own name but the name the
// binary actually looked for never existed — exactly the
// "cannot open shared object file" failure CI hit.
func TestExtractServerBinaryFollowsSymlinkedLibraries(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "llama-b1-bin-ubuntu-x64.tar.gz")
	writeTestTarGzWithSymlinks(t, archive,
		map[string]string{
			"llama-b1/" + serverBinaryName():    "fake server binary",
			"llama-b1/libllama-common.so.0.7.0": "the real shared library bytes",
		},
		map[string]string{
			"llama-b1/libllama-common.so.0": "libllama-common.so.0.7.0", // symlink -> real file
			"llama-b1/libllama-common.so":   "libllama-common.so.0",     // symlink -> symlink
		},
	)

	destDir := filepath.Join(dir, "out")
	if err := extractServerBinary(archive, destDir); err != nil {
		t.Fatalf("extractServerBinary: %v", err)
	}
	// This is the exact name llama-server's dynamic linker looks for —
	// it must resolve to the real library content, not be missing.
	got, err := os.ReadFile(filepath.Join(destDir, "libllama-common.so.0"))
	if err != nil {
		t.Fatalf("read libllama-common.so.0 (should resolve through the symlink): %v", err)
	}
	if string(got) != "the real shared library bytes" {
		t.Errorf("libllama-common.so.0 content = %q, want the real library's bytes", got)
	}
}

func writeTestTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()
	writeTestTarGzWithSymlinks(t, path, files, nil)
}

// writeTestTarGzWithSymlinks writes files as regular tar entries and
// symlinks as tar.TypeSymlink entries (value = link target).
func writeTestTarGzWithSymlinks(t *testing.T, path string, files, symlinks map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write tar content: %v", err)
		}
	}
	for name, target := range symlinks {
		hdr := &tar.Header{Name: name, Typeflag: tar.TypeSymlink, Linkname: target, Mode: 0o777}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar symlink header: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
}

// TestPruneToBaselineCPUKernelKeepsOnlyBaseline guards the AMD EPYC
// workaround: with the feature on, every microarchitecture-specific
// ggml-cpu variant must be removed, keeping only the baseline
// llama.cpp's own loader falls back to when no higher-scoring variant
// is present.
func TestPruneToBaselineCPUKernelKeepsOnlyBaseline(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"libggml-cpu-x64.so", // baseline — must survive
		"libggml-cpu-haswell.so",
		"libggml-cpu-zen4.so",
		"libggml-cpu-skylakex.so",
		"llama-server",       // not a ggml-cpu file — must survive
		"libllama-common.so", // not a ggml-cpu file — must survive
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}

	pruneToBaselineCPUKernel(dir, hwdetect.Info{Arch: "amd64"})

	want := map[string]bool{
		"libggml-cpu-x64.so":      true,
		"llama-server":            true,
		"libllama-common.so":      true,
		"libggml-cpu-haswell.so":  false,
		"libggml-cpu-zen4.so":     false,
		"libggml-cpu-skylakex.so": false,
	}
	for name, shouldExist := range want {
		_, err := os.Stat(filepath.Join(dir, name))
		exists := err == nil
		if exists != shouldExist {
			t.Errorf("%s: exists=%v, want %v", name, exists, shouldExist)
		}
	}
}

// TestPruneToBaselineCPUKernelNoOpWithoutBaseline guards against
// pruning everything when the baseline itself isn't present (e.g. a
// build that doesn't split ggml-cpu into variants at all) — better to
// leave every file alone than risk deleting the only kernel available.
func TestPruneToBaselineCPUKernelNoOpWithoutBaseline(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "libggml-cpu-haswell.so"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	pruneToBaselineCPUKernel(dir, hwdetect.Info{Arch: "amd64"})

	if _, err := os.Stat(filepath.Join(dir, "libggml-cpu-haswell.so")); err != nil {
		t.Errorf("expected libggml-cpu-haswell.so to survive when no baseline is present: %v", err)
	}
}

// TestEnsurePrunesCPUKernelsOnlyWhenEnvSet is an end-to-end check that
// Ensure() leaves every extracted ggml-cpu variant alone by default,
// and prunes down to the baseline only when SafeCPUKernelEnv is set —
// normal `runyx` keeps full CPU-optimized performance unchanged.
func TestEnsurePrunesCPUKernelsOnlyWhenEnvSet(t *testing.T) {
	archive := func(dir string) string {
		path := filepath.Join(dir, "llama-b1-bin-ubuntu-x64.tar.gz")
		writeTestTarGz(t, path, map[string]string{
			"llama-b1/" + serverBinaryName():  "fake server binary",
			"llama-b1/libggml-cpu-x64.so":     "baseline",
			"llama-b1/libggml-cpu-haswell.so": "higher-scoring, possibly buggy on some CPUs",
		})
		return path
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghRelease{TagName: "b1", Assets: []ghAsset{{Name: "llama-b1-bin-ubuntu-x64.tar.gz"}}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	archPath := archive(t.TempDir())
	mux.HandleFunc("/dl/llama-b1-bin-ubuntu-x64.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, archPath)
	})
	mux.HandleFunc("/latest2", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghRelease{
			TagName: "b1",
			Assets:  []ghAsset{{Name: "llama-b1-bin-ubuntu-x64.tar.gz", BrowserDownloadURL: srv.URL + "/dl/llama-b1-bin-ubuntu-x64.tar.gz"}},
		})
	})

	old := ReleasesAPI
	ReleasesAPI = srv.URL + "/latest2"
	defer func() { ReleasesAPI = old }()

	info := hwdetect.Info{OS: "linux", Arch: "amd64"}

	t.Run("default: nothing pruned", func(t *testing.T) {
		f := NewFetcher(t.TempDir())
		binPath, err := f.Ensure(context.Background(), info, nil)
		if err != nil {
			t.Fatalf("Ensure: %v", err)
		}
		destDir := filepath.Dir(binPath)
		if _, err := os.Stat(filepath.Join(destDir, "libggml-cpu-haswell.so")); err != nil {
			t.Errorf("expected libggml-cpu-haswell.so to survive by default: %v", err)
		}
	})

	t.Run("env set: pruned to baseline", func(t *testing.T) {
		t.Setenv(SafeCPUKernelEnv, "1")
		f := NewFetcher(t.TempDir())
		binPath, err := f.Ensure(context.Background(), info, nil)
		if err != nil {
			t.Fatalf("Ensure: %v", err)
		}
		destDir := filepath.Dir(binPath)
		if _, err := os.Stat(filepath.Join(destDir, "libggml-cpu-haswell.so")); !os.IsNotExist(err) {
			t.Errorf("expected libggml-cpu-haswell.so to be pruned when %s is set", SafeCPUKernelEnv)
		}
		if _, err := os.Stat(filepath.Join(destDir, "libggml-cpu-x64.so")); err != nil {
			t.Errorf("expected the baseline libggml-cpu-x64.so to survive: %v", err)
		}
	})
}

// TestVerifyOrPinBinaryTrustsOnFirstUse covers an install from before
// this hash-pinning existed: a binary with no .sha256 sidecar yet must
// not be rejected outright (that would break every existing cache),
// but gets pinned on this first check so tampering after this point is
// still caught.
func TestVerifyOrPinBinaryTrustsOnFirstUse(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, serverBinaryName())
	os.WriteFile(binPath, []byte("a binary"), 0o755)

	if !verifyOrPinBinary(binPath) {
		t.Fatal("expected a binary with no prior pin to be trusted (and pinned) on first use")
	}
	if _, err := os.Stat(pinHashPath(binPath)); err != nil {
		t.Error("expected a .sha256 sidecar to be written after trust-on-first-use")
	}
}

// TestVerifyOrPinBinaryDetectsTampering is the actual guarantee this
// mechanism exists for: once pinned, a binary that changes on disk --
// tampering, corruption, a hand-placed substitute -- must be detected,
// not silently trusted the way a bare os.Stat (what Ensure did before
// this fix) would.
func TestVerifyOrPinBinaryDetectsTampering(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, serverBinaryName())
	os.WriteFile(binPath, []byte("the real binary"), 0o755)
	if !verifyOrPinBinary(binPath) {
		t.Fatal("expected first-use pinning to succeed")
	}

	// Simulate tampering after the pin was recorded.
	os.WriteFile(binPath, []byte("a substituted binary"), 0o755)

	if verifyOrPinBinary(binPath) {
		t.Error("expected a binary that changed after being pinned to fail verification")
	}
}

func TestVerifyDigestAcceptsMatchingSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.tar.gz")
	os.WriteFile(path, []byte("archive contents"), 0o644)
	sum, err := sha256Hex(path)
	if err != nil {
		t.Fatalf("sha256Hex: %v", err)
	}
	if err := verifyDigest(path, "sha256:"+sum); err != nil {
		t.Errorf("verifyDigest with a matching digest: %v", err)
	}
}

func TestVerifyDigestRejectsMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.tar.gz")
	os.WriteFile(path, []byte("archive contents"), 0o644)
	if err := verifyDigest(path, "sha256:0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Error("expected a mismatched digest to be rejected")
	}
}

// TestEnsureRedownloadsWhenCachedBinaryWasTampered is the end-to-end
// version of TestVerifyOrPinBinaryDetectsTampering, reusing the same
// releases-API-unreachable path TestEnsureFallsBackToCachedBinaryWhenReleasesAPIFails
// does: a cached binary that was pinned once and then modified on disk
// must not be returned by Ensure() as a trusted result.
func TestEnsureRedownloadsWhenCachedBinaryWasTampered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	old := ReleasesAPI
	ReleasesAPI = srv.URL
	defer func() { ReleasesAPI = old }()

	dir := t.TempDir()
	info := hwdetect.Info{OS: "linux", Arch: "amd64"}
	cachedDir := filepath.Join(dir, "b1234-linux-amd64")
	os.MkdirAll(cachedDir, 0o755)
	binPath := filepath.Join(cachedDir, serverBinaryName())
	os.WriteFile(binPath, []byte("the pinned binary"), 0o755)
	if !verifyOrPinBinary(binPath) {
		t.Fatal("expected initial pin to succeed")
	}
	// Tamper with it after pinning.
	os.WriteFile(binPath, []byte("a substituted binary"), 0o755)

	f := NewFetcher(dir)
	_, err := f.Ensure(context.Background(), info, nil)
	// With the releases API unreachable and no genuinely-cached binary
	// left (the only candidate just failed verification), Ensure must
	// fail rather than fall back to the tampered file.
	if err == nil {
		t.Error("expected Ensure to refuse a tampered cached binary when no fresh download is reachable")
	}
}
