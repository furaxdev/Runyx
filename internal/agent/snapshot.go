package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ensureGitRepo makes sure dir is a git repository, initializing one
// (with an initial commit so there's a baseline to diff/reset against)
// if it isn't already — this is what makes "undo via git snapshot"
// possible even in a directory the user never ran `git init` in.
func ensureGitRepo(ctx context.Context, dir string) error {
	if isGitRepo(ctx, dir) {
		return nil
	}
	if err := runGit(ctx, dir, "init"); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	if err := runGit(ctx, dir, "config", "user.email", "runyx@localhost"); err != nil {
		return err
	}
	if err := runGit(ctx, dir, "config", "user.name", "Runyx Agent"); err != nil {
		return err
	}
	if err := runGit(ctx, dir, "add", "-A"); err != nil {
		return err
	}
	// --allow-empty: a brand-new directory may have nothing to add yet.
	if err := runGit(ctx, dir, "commit", "--allow-empty", "-m", "runyx: initial snapshot"); err != nil {
		return fmt.Errorf("initial commit: %w", err)
	}
	return nil
}

func isGitRepo(ctx context.Context, dir string) bool {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	return cmd.Run() == nil
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Snapshot commits the working tree's current state (including
// untracked files) under a label, returning the commit hash so a later
// Undo can target it precisely.
func Snapshot(ctx context.Context, dir, label string) (string, error) {
	if err := ensureGitRepo(ctx, dir); err != nil {
		return "", err
	}
	if err := runGit(ctx, dir, "add", "-A"); err != nil {
		return "", err
	}
	if err := runGit(ctx, dir, "commit", "--allow-empty", "-m", "runyx snapshot: "+label); err != nil {
		return "", err
	}
	return gitOutput(ctx, dir, "rev-parse", "HEAD")
}

// Diff returns the working tree's changes since a prior snapshot commit
// (or since HEAD if since is empty), for the "diff preview" requirement
// — shown to the user before/after a command runs.
func Diff(ctx context.Context, dir, since string) (string, error) {
	if since == "" {
		since = "HEAD"
	}
	if err := runGit(ctx, dir, "add", "-A"); err != nil {
		return "", err
	}
	out, err := gitOutput(ctx, dir, "diff", "--cached", since)
	if err != nil {
		return "", err
	}
	// Undo the staging Diff performed just to compute it, so it doesn't
	// change repo state as a side effect of merely inspecting it.
	runGit(ctx, dir, "reset")
	return out, nil
}

// Undo hard-resets the working tree to a prior snapshot commit. This is
// the only operation in this package allowed to discard uncommitted
// changes outside WorkDir's own history — and even then only within
// WorkDir, since every git command here runs with Dir=WorkDir.
func Undo(ctx context.Context, dir, commitHash string) error {
	return runGit(ctx, dir, "reset", "--hard", commitHash)
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// mustAbs is a small helper used by callers needing a clean absolute
// path before running a command, kept here since snapshot.go already
// imports path/filepath and os for directory checks.
func mustAbs(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", abs)
	}
	return abs, nil
}
