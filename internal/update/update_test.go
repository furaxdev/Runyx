package update

import (
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyRealEd25519Signature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	oldKey := PublicKeyHex
	PublicKeyHex = hex.EncodeToString(pub)
	defer func() { PublicKeyHex = oldKey }()

	binary := []byte("fake binary contents for testing purposes")
	sig := ed25519.Sign(priv, binary)

	if err := Verify(binary, hex.EncodeToString(sig)); err != nil {
		t.Errorf("Verify of a correctly signed binary failed: %v", err)
	}
}

func TestVerifyRejectsTamperedBinary(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	oldKey := PublicKeyHex
	PublicKeyHex = hex.EncodeToString(pub)
	defer func() { PublicKeyHex = oldKey }()

	binary := []byte("original content")
	sig := ed25519.Sign(priv, binary)

	tampered := []byte("tampered content!")
	if err := Verify(tampered, hex.EncodeToString(sig)); err == nil {
		t.Error("expected verification to fail for tampered binary content")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	otherPub, _, _ := ed25519.GenerateKey(nil) // a different key pair entirely

	oldKey := PublicKeyHex
	PublicKeyHex = hex.EncodeToString(otherPub)
	defer func() { PublicKeyHex = oldKey }()

	binary := []byte("content")
	sig := ed25519.Sign(priv, binary) // signed with the WRONG key relative to PublicKeyHex

	if err := Verify(binary, hex.EncodeToString(sig)); err == nil {
		t.Error("expected verification to fail when signed by a different key")
	}
}

func TestVerifyRefusesWithNoKeyConfigured(t *testing.T) {
	oldKey := PublicKeyHex
	PublicKeyHex = ""
	defer func() { PublicKeyHex = oldKey }()

	if err := Verify([]byte("x"), "00"); err == nil {
		t.Error("expected Verify to refuse (not silently pass) with no public key configured")
	}
}

func TestApplyAndRollbackOnRealFiles(t *testing.T) {
	dir := t.TempDir()
	exePath := filepath.Join(dir, "runyx")
	originalContent := []byte("ORIGINAL BINARY BYTES")
	if err := os.WriteFile(exePath, originalContent, 0o755); err != nil {
		t.Fatal(err)
	}

	newContent := []byte("NEW BINARY BYTES, VERSION 2")
	backup, err := Apply(exePath, newContent)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got, _ := os.ReadFile(exePath)
	if string(got) != string(newContent) {
		t.Errorf("after Apply, exe content = %q, want %q", got, newContent)
	}

	backedUp, _ := os.ReadFile(backup)
	if string(backedUp) != string(originalContent) {
		t.Errorf("backup content = %q, want original %q", backedUp, originalContent)
	}

	if err := Rollback(exePath, backup); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	restored, _ := os.ReadFile(exePath)
	if string(restored) != string(originalContent) {
		t.Errorf("after Rollback, exe content = %q, want original %q", restored, originalContent)
	}
}

func TestPortableDataDirRespectsEnvVar(t *testing.T) {
	t.Setenv("RUNYX_PORTABLE", "")
	if got := PortableDataDir(); got != "" {
		t.Errorf("PortableDataDir() = %q without RUNYX_PORTABLE set, want empty", got)
	}

	t.Setenv("RUNYX_PORTABLE", "1")
	if got := PortableDataDir(); got == "" {
		t.Error("PortableDataDir() should return a path when RUNYX_PORTABLE=1")
	}
}
