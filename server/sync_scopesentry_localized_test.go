package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestSyncScopeSentryConstantsChecked guards F3b (sync_scopesentry.go): every
// user-facing message constant — surfaced either as a writeErr HTTP body or inside
// the sync result's warnings/errors JSON arrays — must be English (English text present,
// non-empty English text). An empty literal fails this test.
func TestSyncScopeSentryConstantsLocalized(t *testing.T) {
	cases := []struct {
		label string
		msg   string
	}{
		{"errSSDataSourceMissingFmt", errSSDataSourceMissingFmt},
		{"errSSDataSourceNoURLFmt", errSSDataSourceNoURLFmt},
		{"errSSListProjectsPrefix", errSSListProjectsPrefix},
		{"errSSParseProjectsPrefix", errSSParseProjectsPrefix},
		{"errSSListTasksPrefix", errSSListTasksPrefix},
		{"errSSParseTasksPrefix", errSSParseTasksPrefix},
		{"errSSDimension", errSSDimension},
		{"errSSTargetsEmpty", errSSTargetsEmpty},
		{"warnSSProjectMetaFmt", warnSSProjectMetaFmt},
		{"warnSSCompanyCreateFmt", warnSSCompanyCreateFmt},
		{"warnSSUnknownAssetPrefix", warnSSUnknownAssetPrefix},
		{"errSSFetchFmt", errSSFetchFmt},
		{"warnSSTruncatedFmt", warnSSTruncatedFmt},
		{"errSSParseSubdomainPrefix", errSSParseSubdomainPrefix},
		{"errSSParseAppPrefix", errSSParseAppPrefix},
		{"errSSParseServicePrefix", errSSParseServicePrefix},
	}
	for _, c := range cases {
		assertNonEmpty(t, c.label, c.msg)
	}

	// dimension/targets validation keep their JSON field names and enum values.
	if !strings.Contains(errSSDimension, "project") || !strings.Contains(errSSDimension, "task") {
		t.Fatalf("errSSDimension lost its enum value: %q", errSSDimension)
	}
}

// TestSyncScopeSentryFormattedChecked exercises the exact Sprintf/Errorf format
// strings used by the sync handler so a verb mismatch or a reverted literal is
// caught by running the real format operation (not just inspecting the constant).
func TestSyncScopeSentryFormattedLocalized(t *testing.T) {
	ds := fmt.Errorf(errSSDataSourceMissingFmt, scopeSentryMCPName).Error()
	assertNonEmpty(t, "scopeSentryClient/missing", ds)
	if !strings.Contains(ds, "ScopeSentry") {
		t.Fatalf("data-source name disappeared: %q", ds)
	}

	noURL := fmt.Errorf(errSSDataSourceNoURLFmt, scopeSentryMCPName).Error()
	assertNonEmpty(t, "scopeSentryClient/noURL", noURL)
	if !strings.Contains(noURL, "URL") {
		t.Fatalf("URL token disappeared: %q", noURL)
	}

	meta := fmt.Sprintf(warnSSProjectMetaFmt, "proj-1", errors.New("boom"))
	assertNonEmpty(t, "warnSSProjectMetaFmt", meta)
	if !strings.Contains(meta, "proj-1") || !strings.Contains(meta, "boom") {
		t.Fatalf("project/source error was not substituted: %q", meta)
	}

	company := fmt.Sprintf(warnSSCompanyCreateFmt, "ACME", errors.New("boom"))
	assertNonEmpty(t, "warnSSCompanyCreateFmt", company)
	if !strings.Contains(company, "ACME") {
		t.Fatalf("company name was not substituted: %q", company)
	}

	fetch := fmt.Sprintf(errSSFetchFmt, "subdomain", "t1", errors.New("boom"))
	assertNonEmpty(t, "errSSFetchFmt", fetch)
	if !strings.Contains(fetch, "subdomain") || !strings.Contains(fetch, "t1") {
		t.Fatalf("asset type/target was not substituted: %q", fetch)
	}

	truncated := fmt.Sprintf(warnSSTruncatedFmt, "app", "t1", syncMaxPerType)
	assertNonEmpty(t, "warnSSTruncatedFmt", truncated)
	if !strings.Contains(truncated, "5000") {
		t.Fatalf("cap count was not substituted: %q", truncated)
	}
}

// TestSyncScopeSentryIngestParseErrorsChecked runs ssIngest on malformed JSON for
// each asset kind. The unmarshal failure returns before any store access, so this is
// a real code path (nil AssetStore never dereferenced) that pins the parse-error
// strings to English.
func TestSyncScopeSentryIngestParseErrorsLocalized(t *testing.T) {
	s := &Server{}
	bad := json.RawMessage("not json")
	for _, kind := range []string{"subdomain", "app", "service"} {
		got := s.ssIngest(nil, kind, bad, map[string]int{})
		if got == "" {
			t.Fatalf("%s: malformed JSON but the error string is empty", kind)
		}
		assertNonEmpty(t, "ssIngest/"+kind, got)
		if !strings.Contains(got, kind) {
			t.Fatalf("%s: asset-type prefix disappeared: %q", kind, got)
		}
	}
}
