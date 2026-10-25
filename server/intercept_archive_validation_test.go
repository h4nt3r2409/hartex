package server

import (
	"net/url"
	"testing"
)

// TestInterceptFilterParamValidation covers the query-string validation done by
// interceptFilterParams. These validators do not touch the database.
func TestInterceptFilterParamValidation(t *testing.T) {
	if _, err := interceptFilterParams(url.Values{"status": {"bogus"}}); err == nil {
		t.Fatal("bogus status must not pass validation")
	}
	if _, err := interceptFilterParams(url.Values{"decision_source": {"bogus"}}); err == nil {
		t.Fatal("bogus decision_source must not pass validation")
	}
}

// TestInterceptRuleValidation pins each rejection branch of validateInterceptRuleReq
// by making the earlier fields valid.
func TestInterceptRuleValidation(t *testing.T) {
	cases := []struct {
		label string
		req   interceptRuleReq
	}{
		{"rule.name", interceptRuleReq{Name: ""}},
		{"rule.match_target", interceptRuleReq{Name: "r", MatchTarget: "bad"}},
		{"rule.match_type", interceptRuleReq{Name: "r", MatchTarget: "tool_name", MatchType: "bad"}},
		{"rule.pattern_empty", interceptRuleReq{Name: "r", MatchTarget: "tool_name", MatchType: "string", Pattern: ""}},
		{"rule.action", interceptRuleReq{Name: "r", MatchTarget: "tool_name", MatchType: "string", Pattern: "p", Action: "bad"}},
		{"rule.regex", interceptRuleReq{Name: "r", MatchTarget: "tool_name", MatchType: "regex", Pattern: "(", Action: "allow"}},
	}
	for _, tc := range cases {
		if err := validateInterceptRuleReq(tc.req); err == nil {
			t.Fatalf("%s: validation must not pass", tc.label)
		}
	}
}

// TestNormalizeArchiveIDs covers the batch id bounds validation.
func TestNormalizeArchiveIDs(t *testing.T) {
	if _, err := normalizeArchiveIDs(nil); err == nil {
		t.Fatal("an empty archive_ids list must not pass validation")
	}
	tooMany := make([]int64, 101)
	for i := range tooMany {
		tooMany[i] = int64(i + 1)
	}
	if _, err := normalizeArchiveIDs(tooMany); err == nil {
		t.Fatal("101 archive_ids must not pass validation")
	}
	if _, err := normalizeArchiveIDs([]int64{0}); err == nil {
		t.Fatal("a zero archive id must not pass validation")
	}
}

// TestValidateArchivePath covers the archive package-path boundaries.
func TestValidateArchivePath(t *testing.T) {
	if err := validateArchivePath("/tmp/artex-data", ""); err == nil {
		t.Fatal("an empty archive path must not pass validation")
	}
	if err := validateArchivePath("/tmp/artex-data", "/etc/passwd"); err == nil {
		t.Fatal("a path outside the managed directory must not pass validation")
	}
}
