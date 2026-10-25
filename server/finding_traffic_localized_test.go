package server

import "testing"

// A regression guard test that keeps the user-facing HTTP error response messages
// in finding_traffic.go in English. The check reuses assertNonEmpty
// (non-empty messages). The four paths
// that return these messages (findingTrafficAccess, bindFindingTraffic,
// editFindingTraffic) are all behind the s.m.pg.GetFinding (DB) gate and cannot run
// to completion on this host without a database, so the constants themselves are
// pinned (the same approach as the DB-gated paths in goals_api and notify_api). It
// fails if a message becomes empty.
//
// The errors.New calls (offset/length validation) and binary-body placeholder in
// readEvidencePreview, and the roTool("get_finding_traffic") tool description, are
// brain input fed back as agent tool results, so they intentionally preserve the
// original Chinese text and are not assertions in this test.
func TestFindingTrafficErrorsLocalized(t *testing.T) {
	for _, c := range []struct {
		label, msg string
	}{
		{"inherited_readonly", errFindingTrafficInheritedReadonly},
		{"select_required", errFindingTrafficSelectRequired},
		{"version_required", errFindingTrafficVersionRequired},
		{"binding_ids_required", errFindingTrafficBindingIDsRequired},
	} {
		assertNonEmpty(t, c.label, c.msg)
	}
}
