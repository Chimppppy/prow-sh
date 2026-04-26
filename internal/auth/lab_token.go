package auth

import (
	"crypto/subtle"
	"os"
	"strings"
)

// ReadLabTokenFile reads a single-line token from disk (Phase 0A lab).
func ReadLabTokenFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// BearerMatches returns true if got matches want using constant-time compare.
func BearerMatches(got, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
