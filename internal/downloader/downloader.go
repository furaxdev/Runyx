// Package downloader implements resumable, parallel, checksum-verified
// file downloads, used for both llama.cpp backend builds and GGUF model
// files pulled from Hugging Face.
package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sync/errgroup"
)

// Progress reports download state for a ProgressFunc callback.
type Progress struct {
	Downloaded int64
	Total      int64 // 0 if unknown
}

// ProgressFunc is called periodically as bytes land on disk. Even with
// Concurrency > 1, calls are serialized by the downloader (never made
// concurrently to each other), so implementations don't need their own
// locking.
type ProgressFunc func(Progress)

// Options configures a Download call.
type Options struct {
	// Concurrency is the number of parallel range requests to use when the
	// server advertises range support. 1 disables chunking.
	Concurrency int
	// ExpectedSHA256, if set, is verified after a complete download;
	// mismatch returns ErrChecksumMismatch and the file is NOT deleted,
	// so callers can inspect it (but should not trust its contents).
	ExpectedSHA256 string
	// OnProgress is invoked as bytes are written. May be nil.
	OnProgress ProgressFunc
	// MinFreeBytes, if > 0, is checked against the destination's free
	// disk space before downloading starts (in addition to the content
	// length itself, when known).
	MinFreeBytes int64
}

// ErrChecksumMismatch is returned when the downloaded file's SHA256 does
// not match Options.ExpectedSHA256.
var ErrChecksumMismatch = errors.New("downloaded file failed checksum verification")

// ErrInsufficientDiskSpace is returned when the destination volume doesn't
// have enough free space for the download.
var ErrInsufficientDiskSpace = errors.New("insufficient disk space")

type resumeState struct {
	URL          string  `json:"url"`
	TotalSize    int64   `json:"total_size"`
	ChunkOffsets []int64 `json:"chunk_offsets"`
	ChunkDone    []bool  `json:"chunk_done"`
}

func statePath(dest string) string { return dest + ".runyx-download.json" }
func partPath(dest string) string  { return dest + ".part" }

// Download fetches url into dest, resuming a previous partial download if
// one exists at dest+".part" with matching metadata. On success, dest
// exists at its final path and all temp/state files are cleaned up.
func Download(ctx context.Context, client *http.Client, url, dest string, opts Options) error {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}
	if client == nil {
		client = http.DefaultClient
	}

	total, acceptsRanges, err := probe(ctx, client, url)
	if err != nil {
		return fmt.Errorf("probe %s: %w", url, err)
	}

	if err := checkDiskSpace(filepath.Dir(dest), maxInt64(total, opts.MinFreeBytes)); err != nil {
		return err
	}

	part := partPath(dest)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	if !acceptsRanges || opts.Concurrency == 1 || total <= 0 {
		if err := downloadSingleStream(ctx, client, url, part, opts.OnProgress, total); err != nil {
			return err
		}
	} else {
		if err := downloadChunked(ctx, client, url, part, total, opts); err != nil {
			return err
		}
	}

	if opts.ExpectedSHA256 != "" {
		sum, err := sha256File(part)
		if err != nil {
			return fmt.Errorf("checksum: %w", err)
		}
		if sum != opts.ExpectedSHA256 {
			return fmt.Errorf("%w: got %s, want %s", ErrChecksumMismatch, sum, opts.ExpectedSHA256)
		}
	}

	os.Remove(statePath(part))
	return os.Rename(part, dest)
}

// probe issues a HEAD (falling back to a ranged GET if HEAD is rejected)
// to learn content length and whether byte ranges are supported.
func probe(ctx context.Context, client *http.Client, url string) (total int64, acceptsRanges bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return 0, false, fmt.Errorf("HEAD %s: status %d", url, resp.StatusCode)
	}
	return resp.ContentLength, resp.Header.Get("Accept-Ranges") == "bytes", nil
}

func downloadSingleStream(ctx context.Context, client *http.Client, url, part string, onProgress ProgressFunc, total int64) error {
	var startAt int64
	if info, err := os.Stat(part); err == nil {
		startAt = info.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if startAt > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startAt))
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		// Server ignored our Range header (or we had nothing to resume);
		// start clean.
		flags |= os.O_TRUNC
		startAt = 0
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}

	f, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	downloaded := startAt
	w := &progressWriter{w: f, onProgress: func(n int64) {
		downloaded += n
		if onProgress != nil {
			onProgress(Progress{Downloaded: downloaded, Total: total})
		}
	}}
	_, err = io.Copy(w, resp.Body)
	return err
}

// downloadChunked splits [0, total) into opts.Concurrency byte ranges and
// fetches them concurrently into a preallocated file, tracking per-chunk
// completion in a sidecar JSON file so an interrupted download resumes
// without re-fetching finished chunks.
func downloadChunked(ctx context.Context, client *http.Client, url, part string, total int64, opts Options) error {
	state, err := loadOrInitState(part, url, total, opts.Concurrency)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err := f.Truncate(total); err != nil {
		f.Close()
		return err
	}
	defer f.Close()

	var mu sync.Mutex
	var downloaded int64
	for i, done := range state.ChunkDone {
		if done {
			end := total
			if i+1 < len(state.ChunkOffsets) {
				end = state.ChunkOffsets[i+1]
			}
			downloaded += end - state.ChunkOffsets[i]
		}
	}

	g, ctx := errgroup.WithContext(ctx)
	for i := range state.ChunkOffsets {
		i := i
		if state.ChunkDone[i] {
			continue
		}
		start := state.ChunkOffsets[i]
		end := total - 1
		if i+1 < len(state.ChunkOffsets) {
			end = state.ChunkOffsets[i+1] - 1
		}
		g.Go(func() error {
			if err := fetchRange(ctx, client, url, f, start, end, func(n int64) {
				// Hold the lock across the callback itself, not just the
				// counter update: callers' OnProgress implementations
				// (including this CLI's own) aren't all written to be
				// safe against concurrent invocation, so serialize calls
				// here instead of pushing that burden onto every caller.
				mu.Lock()
				defer mu.Unlock()
				downloaded += n
				if opts.OnProgress != nil {
					opts.OnProgress(Progress{Downloaded: downloaded, Total: total})
				}
			}); err != nil {
				return err
			}
			mu.Lock()
			state.ChunkDone[i] = true
			_ = saveState(part, state)
			mu.Unlock()
			return nil
		})
	}
	return g.Wait()
}

func fetchRange(ctx context.Context, client *http.Client, url string, f *os.File, start, end int64, onN func(int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s range %d-%d: status %d", url, start, end, resp.StatusCode)
	}
	offset := start
	buf := make([]byte, 256*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.WriteAt(buf[:n], offset); werr != nil {
				return werr
			}
			offset += int64(n)
			onN(int64(n))
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func loadOrInitState(part, url string, total int64, concurrency int) (resumeState, error) {
	if data, err := os.ReadFile(statePath(part)); err == nil {
		var s resumeState
		if json.Unmarshal(data, &s) == nil && s.URL == url && s.TotalSize == total {
			return s, nil
		}
	}
	chunkSize := total / int64(concurrency)
	if chunkSize == 0 {
		chunkSize = total
	}
	var offsets []int64
	for off := int64(0); off < total; off += chunkSize {
		offsets = append(offsets, off)
	}
	s := resumeState{
		URL:          url,
		TotalSize:    total,
		ChunkOffsets: offsets,
		ChunkDone:    make([]bool, len(offsets)),
	}
	return s, saveState(part, s)
}

func saveState(part string, s resumeState) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(part), data, 0o644)
}

type progressWriter struct {
	w          io.Writer
	onProgress func(int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	if n > 0 && p.onProgress != nil {
		p.onProgress(int64(n))
	}
	return n, err
}

func sha256File(path string) (string, error) {
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

func checkDiskSpace(dir string, needed int64) error {
	if needed <= 0 {
		return nil
	}
	free, err := freeDiskSpace(dir)
	if err != nil {
		return nil // best-effort; don't block downloads if we can't stat
	}
	// Require some margin beyond the exact byte count.
	if free < needed+needed/20 {
		return fmt.Errorf("%w: need ~%d MB, have %d MB free in %s",
			ErrInsufficientDiskSpace, needed/(1024*1024), free/(1024*1024), dir)
	}
	return nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
