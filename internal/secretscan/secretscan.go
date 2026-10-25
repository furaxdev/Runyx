// Package secretscan redacts secret-shaped substrings (API keys,
// tokens, private key blocks) from text before it's shown to a UI,
// written to memory or a session, or sent back to a model. It is
// deliberately pattern-based and conservative: false positives (over-
// redacting) are an acceptable cost; a real secret slipping through is
// not. Used wherever Runyx Code's threat model calls for it (see
// docs/RUNYX_CODE.md's threat model table) -- command output, file
// reads, and every memory write.
package secretscan

import "regexp"

// pattern pairs a detector with the label substituted into its match
// (e.g. "[redacted:github-token]"), so a human reading redacted output
// still knows roughly what kind of secret was there.
type pattern struct {
	label string
	re    *regexp.Regexp
}

var patterns = []pattern{
	// GitHub tokens: ghp_/gho_/ghu_/ghs_/ghr_/github_pat_ prefixes are
	// a hard signal — GitHub's own token format, not a heuristic.
	{"github-token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,255}\b`)},
	{"github-token", regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{22,255}\b`)},
	// AWS access key id + secret key.
	{"aws-access-key-id", regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"aws-secret-key", regexp.MustCompile(`(?i)aws_secret_access_key\s*[:=]\s*['"]?([A-Za-z0-9/+=]{40})['"]?`)},
	// Generic Bearer / Authorization header tokens.
	{"bearer-token", regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9\-._~+/]{20,}=*`)},
	// OpenAI-style secret keys.
	{"api-key", regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`)},
	// Slack tokens.
	{"slack-token", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`)},
	// Generic "password/secret/token = ..." assignments in config-like
	// text (env files, shell exports, YAML/TOML/JSON single-line).
	{"credential", regexp.MustCompile(`(?i)\b(password|passwd|secret|token|api[_-]?key)\s*[:=]\s*['"]?[^\s'"]{8,}['"]?`)},
	// PEM private key blocks (the whole block, not just the header).
	{"private-key", regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)},
}

// Redact replaces every secret-shaped match in text with
// "[redacted:<label>]", returning the redacted text and whether
// anything was found.
func Redact(text string) (redacted string, found bool) {
	out := text
	for _, p := range patterns {
		if p.re.MatchString(out) {
			found = true
			out = p.re.ReplaceAllString(out, "[redacted:"+p.label+"]")
		}
	}
	return out, found
}

// Contains reports whether text has anything that looks like a
// secret, without modifying it -- for a caller that wants to refuse
// outright rather than redact (e.g. "don't even accept this memory
// entry").
func Contains(text string) bool {
	for _, p := range patterns {
		if p.re.MatchString(text) {
			return true
		}
	}
	return false
}
