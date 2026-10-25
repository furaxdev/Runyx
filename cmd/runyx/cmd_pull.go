package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/furaxdev/runyx/internal/downloader"
)

// cmdPull downloads a GGUF model from Hugging Face. Accepts either
// "owner/repo" (picks the most appropriate .gguf file, preferring
// Q4_K_M-style quantizations) or "owner/repo:filename.gguf" for an exact
// file.
func cmdPull(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: runyx pull <owner/repo>[:file.gguf]")
	}
	repo, file := splitRepoFile(args[0])

	_, store, err := loadStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	if file == "" {
		files, err := downloader.ListGGUFFiles(ctx, nil, repo)
		if err != nil {
			return fmt.Errorf("list files in %s: %w", repo, err)
		}
		if len(files) == 0 {
			return fmt.Errorf("no .gguf files found in %s", repo)
		}
		file = pickBestGGUF(files)
		fmt.Printf("Selected %s (use %s:<file> to pick a different quantization)\n", file, repo)
	}

	url := downloader.FileURL(repo, file)
	dest := store.DestPath(repo, file)

	name := args[0]
	fmt.Printf("Pulling %s...\n", name)
	start := time.Now()
	lastPrint := time.Now()
	err = downloader.Download(ctx, nil, url, dest, downloader.Options{
		Concurrency: 4,
		OnProgress: func(p downloader.Progress) {
			if time.Since(lastPrint) < 200*time.Millisecond {
				return
			}
			lastPrint = time.Now()
			printProgress(p)
		},
	})
	fmt.Println()
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	fmt.Printf("Downloaded in %s\n", time.Since(start).Round(time.Second))

	mdl, err := store.ImportLocal(name, dest)
	if err != nil {
		return fmt.Errorf("register model: %w", err)
	}
	fmt.Printf("Installed as %q (%s)\n", mdl.Name, humanBytes(mdl.SizeBytes))
	return nil
}

func splitRepoFile(spec string) (repo, file string) {
	if idx := strings.LastIndex(spec, ":"); idx >= 0 && !strings.Contains(spec[idx:], "/") {
		return spec[:idx], spec[idx+1:]
	}
	return spec, ""
}

// pickBestGGUF prefers Q4_K_M, then falls back to the first file
// alphabetically so the choice is at least deterministic.
func pickBestGGUF(files []string) string {
	preferred := []string{"q4_k_m", "q4_k_s", "q5_k_m", "q8_0"}
	lower := make(map[string]string, len(files))
	for _, f := range files {
		lower[strings.ToLower(f)] = f
	}
	for _, p := range preferred {
		for lf, orig := range lower {
			if strings.Contains(lf, p) {
				return orig
			}
		}
	}
	sorted := append([]string{}, files...)
	sort.Strings(sorted)
	return sorted[0]
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func printProgress(p downloader.Progress) {
	if p.Total <= 0 {
		fmt.Fprintf(os.Stderr, "\r%s downloaded", humanBytes(p.Downloaded))
		return
	}
	pct := float64(p.Downloaded) / float64(p.Total) * 100
	fmt.Fprintf(os.Stderr, "\r%6.1f%%  %s / %s", pct, humanBytes(p.Downloaded), humanBytes(p.Total))
}
