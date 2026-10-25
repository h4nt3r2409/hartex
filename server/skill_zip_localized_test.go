package server

import (
	"fmt"
	"strings"
	"testing"
)

// TestSkillZipErrors guards the skill-upload error responses produced by
// skill_zip.go. fsUploadSkill surfaces these via writeErr(400, err.Error()) in
// server_mgmt.go.
func TestSkillZipErrors(t *testing.T) {
	// Message constants rendered with sample arguments.
	assertNonEmpty(t, "errSkillZipParse",
		fmt.Errorf(errSkillZipParse, fmt.Errorf("boom")).Error())
	assertNonEmpty(t, "errSkillZipEncrypted",
		fmt.Sprintf(errSkillZipEncrypted, "demo/SKILL.md"))
	assertNonEmpty(t, "errSkillZipUnsupported",
		fmt.Sprintf(errSkillZipUnsupported, "LZMA", 14, "demo/SKILL.md"))

	// A non-zip upload must reach the user as a parse hint, not a raw stdlib error.
	if _, err := newSkillZipReader([]byte("this is not a zip archive")); err == nil {
		t.Fatal("non-zip input must return an error")
	} else {
		assertNonEmpty(t, "newSkillZipReader", err.Error())
		if !strings.Contains(err.Error(), "zip format") {
			t.Fatalf("parse error = %q, want a 'zip format' hint", err.Error())
		}
	}

	// Known compression methods resolve to their display names; unknown methods
	// fall back to "unknown".
	if got := zipMethodName(zipMethodAES); got != "AES encryption" {
		t.Fatalf("zipMethodName(AES) = %q, want %q", got, "AES encryption")
	}
	if got := zipMethodName(0xffff); got != "unknown" {
		t.Fatalf("zipMethodName(unknown) = %q, want %q", got, "unknown")
	}
}
