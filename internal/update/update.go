// Package update implements signed self-updates (item 30): checking
// GitHub for a newer release, downloading the binary for the current
// OS/arch, verifying an Ed25519 signature against a public key
// embedded in the binary, and atomically replacing the running
// executable — with a rollback copy kept in case the new binary fails
// to start.
package update

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

// PublicKeyHex is the Ed25519 public key (hex-encoded) releases are
// signed with. Empty until a real release key pair exists; Verify
// refuses to pass with an empty key rather than silently accepting
// anything — see docs/DECISIONS.md for why no real key is embedded yet.
var PublicKeyHex = ""

// ghAsset/ghRelease mirror the GitHub releases API, same shape as
// internal/backend and internal/voice use for their own fetches.
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

// ReleasesAPI is the GitHub releases endpoint for Runyx itself.
var ReleasesAPI = "https://api.github.com/repos/furaxdev/runyx/releases/latest"

// CheckLatest returns the latest release tag and the asset name
// expected for the current OS/arch (e.g. "runyx-linux-amd64").
func CheckLatest(client *http.Client) (tag string, assetURL string, err error) {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Get(ReleasesAPI)
	if err != nil {
		return "", "", fmt.Errorf("check latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("check latest release: status %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", "", fmt.Errorf("decode release info: %w", err)
	}
	wanted := fmt.Sprintf("runyx-%s-%s", runtime.GOOS, runtime.GOARCH)
	for _, a := range rel.Assets {
		if a.Name == wanted {
			return rel.TagName, a.BrowserDownloadURL, nil
		}
	}
	return rel.TagName, "", fmt.Errorf("no release asset found for %s", wanted)
}

// Verify checks an Ed25519 signature (hex-encoded) over binary's bytes
// against PublicKeyHex. Returns an error — never a silent pass — if no
// public key is configured.
func Verify(binary []byte, signatureHex string) error {
	if PublicKeyHex == "" {
		return fmt.Errorf("no release public key configured; refusing to verify (see docs/DECISIONS.md)")
	}
	pubKeyBytes, err := hex.DecodeString(PublicKeyHex)
	if err != nil {
		return fmt.Errorf("invalid embedded public key: %w", err)
	}
	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}
	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), binary, sigBytes) {
		return fmt.Errorf("signature verification failed: this binary was not signed by the expected release key")
	}
	return nil
}

// Apply atomically replaces currentExe with newBinary, keeping a
// ".bak" copy of the old binary alongside it so a failed start can be
// rolled back by the caller (e.g. the CLI's `runyx update` command,
// which checks the new binary runs `version` successfully before
// deleting the backup).
func Apply(currentExe string, newBinary []byte) (backupPath string, err error) {
	backupPath = currentExe + ".bak"
	if err := copyFile(currentExe, backupPath); err != nil {
		return "", fmt.Errorf("back up current binary: %w", err)
	}

	tmp := currentExe + ".new"
	if err := os.WriteFile(tmp, newBinary, 0o755); err != nil {
		return "", fmt.Errorf("write new binary: %w", err)
	}
	if err := os.Rename(tmp, currentExe); err != nil {
		return "", fmt.Errorf("replace binary: %w", err)
	}
	return backupPath, nil
}

// Rollback restores currentExe from its ".bak" copy.
func Rollback(currentExe, backupPath string) error {
	return os.Rename(backupPath, currentExe)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}

// PortableDataDir returns a data directory rooted next to the running
// executable when RUNYX_PORTABLE=1 is set (item 30's "mode portable":
// everything — config, models, backends — stays in one folder, e.g. on
// a USB stick), or "" when portable mode isn't requested.
func PortableDataDir() string {
	if os.Getenv("RUNYX_PORTABLE") != "1" {
		return ""
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "runyx-data")
}
