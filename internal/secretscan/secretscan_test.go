package secretscan

import (
	"strings"
	"testing"
)

func TestRedactGitHubToken(t *testing.T) {
	in := "export GITHUB_TOKEN=ghp_1234567890abcdefghij1234567890abcdef"
	out, found := Redact(in)
	if !found {
		t.Fatal("expected a GitHub token to be detected")
	}
	if out == in {
		t.Error("expected the token to be redacted")
	}
	if strings.Contains(out, "ghp_1234567890abcdefghij1234567890abcdef") {
		t.Error("raw token still present in redacted output")
	}
}

func TestRedactAWSAccessKeyID(t *testing.T) {
	in := "AKIAIOSFODNN7EXAMPLE is the access key"
	out, found := Redact(in)
	if !found || strings.Contains(out, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("AWS key not redacted: %q", out)
	}
}

func TestRedactPrivateKeyBlock(t *testing.T) {
	in := "-----BEGIN RSA PRIVATE KEY-----\nMIIEpQIBAAKCAQEA\n-----END RSA PRIVATE KEY-----"
	out, found := Redact(in)
	if !found {
		t.Fatal("expected a private key block to be detected")
	}
	if strings.Contains(out, "MIIEpQIBAAKCAQEA") {
		t.Error("private key material still present in redacted output")
	}
}

func TestRedactGenericCredentialAssignment(t *testing.T) {
	in := "password: hunter22222"
	out, found := Redact(in)
	if !found || strings.Contains(out, "hunter22222") {
		t.Errorf("generic credential not redacted: %q", out)
	}
}

func TestRedactLeavesOrdinaryTextAlone(t *testing.T) {
	in := "this is a normal sentence about the weather and nothing else"
	out, found := Redact(in)
	if found {
		t.Error("expected no secrets detected in ordinary text")
	}
	if out != in {
		t.Error("expected ordinary text to be returned unchanged")
	}
}

func TestContainsWithoutModifying(t *testing.T) {
	in := "sk-abcdefghijklmnopqrstuvwxyz0123456789"
	if !Contains(in) {
		t.Error("expected Contains to detect an API-key-shaped string")
	}
	if Contains("just a plain sentence") {
		t.Error("expected Contains to be false for ordinary text")
	}
}
