// Command verify-registry checks every entry in models/registry.json
// against the real Hugging Face API: each repo must exist and actually
// contain the declared file, so the catalog never silently drifts from
// reality (the project's "never invent a model" rule). Run it locally
// (`go run ./cmd/verify-registry`) after editing models/registry.json,
// and it also runs in CI on every push/PR that touches the registry.
//
// It is a separate small binary, not a runyx subcommand, since it's a
// maintainer/CI tool that needs outbound network access to
// huggingface.co — something `runyx` itself never requires just to
// start up.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/furaxdev/runyx/internal/registry"
)

func main() {
	reg, err := registry.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "load registry:", err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	ctx := context.Background()

	var failures, warnings int
	for _, m := range reg.Models {
		if err := checkFile(ctx, client, m.Repo, m.File, m.SizeBytes); err != nil {
			fmt.Printf("FAIL  %-40s %v\n", m.ID, err)
			failures++
			continue
		}
		fmt.Printf("OK    %-40s %s\n", m.ID, m.Repo)

		if m.MMProjFile != "" {
			if err := checkFile(ctx, client, m.Repo, m.MMProjFile, 0); err != nil {
				fmt.Printf("FAIL  %-40s mmproj: %v\n", m.ID, err)
				failures++
			}
		}
	}

	fmt.Printf("\n%d models checked, %d failures, %d warnings\n", len(reg.Models), failures, warnings)
	if failures > 0 {
		os.Exit(1)
	}
}

// checkFile HEADs the file's real download URL. A non-200 status is a
// hard failure (the entry no longer exists or was renamed). A declared
// size that drifts from the live Content-Length is only printed, not
// failed on, since repos legitimately re-upload a quantization with a
// slightly different byte count from time to time.
func checkFile(ctx context.Context, client *http.Client, repo, file string, declaredSize int64) error {
	url := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repo, file)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "runyx-verify-registry")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d (repo/file missing, renamed, or now gated)", url, resp.StatusCode)
	}
	if declaredSize > 0 && resp.ContentLength > 0 && resp.ContentLength != declaredSize {
		fmt.Printf("WARN  %-40s size drifted: registry has %d, live is %d\n", repo, declaredSize, resp.ContentLength)
	}
	return nil
}
