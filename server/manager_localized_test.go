package server

import "testing"

// Regression guard that keeps the worker-agent count validation error message in
// manager.go in English. The check reuses assertNonEmpty (contains English text,
// non-empty English text). The SetWorkers(n<=0) branch returns
// before reaching m.pg.SetSetting, so it can actually run with an empty Manager on a
// host without a DB. An empty literal makes this test fail.
//
// Call-graph assessment (see the errWorkersPositive constant comment): this message
// is user-facing only, exposed solely through the settings PUT handler
// (server.go:3512 → writeErr 400), and has no agent tool (actool) path, so it is not
// brain input. The startup recovery error at line 384 of the same file is operator
// startup-log material and outside F3b's scope (original text preserved), so it is
// not the target of this test.
func TestSetWorkersErrorLocalized(t *testing.T) {
	err := (&Manager{}).SetWorkers(0)
	if err == nil {
		t.Fatal("a worker count of 0 must be rejected")
	}
	assertNonEmpty(t, "set_workers_nonpositive", err.Error())
}
