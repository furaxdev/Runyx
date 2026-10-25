package main

import (
	"fmt"
	"os"

	"github.com/furaxdev/runyx/internal/update"
	"github.com/furaxdev/runyx/internal/version"
)

// cmdUpdate checks for (and, once a release signing key exists, applies)
// a newer Runyx release. Applying is refused until update.PublicKeyHex
// is set to a real release key — see docs/DECISIONS.md: shipping a
// "verifies signatures" feature with no real key wired up would be
// worse than not having the feature, since it would look safe while
// actually trusting nothing.
func cmdUpdate(args []string) error {
	tag, assetURL, err := update.CheckLatest(nil)
	if err != nil {
		return fmt.Errorf("check for updates: %w", err)
	}
	fmt.Printf("Current version: %s\n", version.Version)
	fmt.Printf("Latest release:  %s\n", tag)

	if update.PublicKeyHex == "" {
		fmt.Fprintln(os.Stderr, "\nNo release signing key is configured in this build, so `runyx update` cannot safely")
		fmt.Fprintln(os.Stderr, "apply the update automatically (see docs/DECISIONS.md). Download it yourself from:")
		fmt.Fprintf(os.Stderr, "  %s\n", assetURL)
		return nil
	}

	// Reaching here needs a real signing key wired into a release
	// build; not exercised by this session's own dev builds.
	fmt.Println("Downloading and verifying the update...")
	return fmt.Errorf("signed update application is not implemented in this build")
}
