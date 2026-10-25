package agent

import "testing"

// TestMatchesDeniedCatchesPathVariants guards the actual vulnerability
// fixed here: a plain `exe == deny` string comparison lets "sudo" be
// refused while "/bin/sudo", "./sudo", and "SUDO.EXE" -- the exact
// same program -- sail through to the approval prompt, defeating the
// "no sudo, ever, no prompt" guarantee nonNegotiableDenyPrefixes exists
// to give.
func TestMatchesDeniedCatchesPathVariants(t *testing.T) {
	deny := []string{"sudo", "doas", "su"}
	mustDeny := []string{"sudo", "/bin/sudo", "/usr/bin/sudo", "./sudo", "../sudo", "SUDO", "Sudo.exe", "sudo.exe", "doas", "/sbin/doas", "su", "/bin/su"}
	for _, exe := range mustDeny {
		if !matchesDenied(exe, deny) {
			t.Errorf("matchesDenied(%q) = false, want true (bypasses the deny-list)", exe)
		}
	}
}

func TestMatchesDeniedAllowsUnrelatedCommands(t *testing.T) {
	deny := []string{"sudo", "doas", "su"}
	mustAllow := []string{"echo", "git", "sudoku", "subversion", "sue", "ls"}
	for _, exe := range mustAllow {
		if matchesDenied(exe, deny) {
			t.Errorf("matchesDenied(%q) = true, want false (not actually denied, just shares a prefix)", exe)
		}
	}
}

func TestPolicyValidateRefusesSudoPathVariant(t *testing.T) {
	p := Policy{WorkDir: t.TempDir()}
	cases := []string{"/bin/sudo", "./sudo", "SUDO.EXE"}
	for _, exe := range cases {
		if err := p.Validate(Request{Command: exe, WorkDir: p.WorkDir}); err == nil {
			t.Errorf("Validate(%q): expected refusal, got nil error", exe)
		}
	}
}

func TestPolicyValidateRefusesConfiguredDenyPathVariant(t *testing.T) {
	p := Policy{WorkDir: t.TempDir(), DenyPrefixes: []string{"curl"}}
	if err := p.Validate(Request{Command: "/usr/bin/curl", WorkDir: p.WorkDir}); err == nil {
		t.Error("expected a configured deny prefix to also catch an absolute-path variant")
	}
}
