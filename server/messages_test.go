package server

import (
	"strings"
	"testing"
)

// assertNonEmpty guards that a user-facing message constant or rendered response
// body was populated. English is the only language now, so the check verifies the
// message exists without pinning specific wording.
func assertNonEmpty(t *testing.T, label, msg string) {
	t.Helper()
	if strings.TrimSpace(msg) == "" {
		t.Fatalf("%s: empty message", label)
	}
}
