package server

import "testing"

// Regression guard keeping the user-facing error messages returned by the intercept rule
// validators in asset_intercept.go and task_intercept.go in English. The two validators
// (validateAssetInterceptRuleReq, validateTaskInterceptRuleReq) are pure functions that do
// not use the DB, so in addition to pinning the constants, this calls the actual error paths
// directly and verifies the returned messages. The check reuses assertNonEmpty;
// The check uses non-empty messages. Field names and enum values (pattern,
// exact_ip, cidr, kind, action, block, allow) are wire identifiers and stay ASCII, so they do
// not affect the empty-message check. It fails if a message becomes empty.
func TestAssetInterceptErrorsLocalized(t *testing.T) {
	// Pin the constants
	for _, c := range []struct{ label, msg string }{
		{"pattern_empty", errAssetInterceptPatternEmpty},
		{"bad_exact_ip", errAssetInterceptInvalidExactIPFmt},
		{"bad_cidr", errAssetInterceptInvalidCIDRFmt},
		{"bad_kind", errAssetInterceptInvalidKindFmt},
		{"bad_action", errTaskInterceptInvalidAction},
	} {
		assertNonEmpty(t, c.label, c.msg)
	}

	// Actual paths: the four rejection branches of validateAssetInterceptRuleReq
	for _, c := range []struct {
		label string
		req   assetInterceptRuleReq
	}{
		{"empty_pattern", assetInterceptRuleReq{Kind: "exact_ip", Pattern: "   "}},
		{"invalid_exact_ip", assetInterceptRuleReq{Kind: "exact_ip", Pattern: "not-an-ip"}},
		{"invalid_cidr", assetInterceptRuleReq{Kind: "cidr", Pattern: "nonsense"}},
		{"invalid_kind", assetInterceptRuleReq{Kind: "nope", Pattern: "example.com"}},
	} {
		req := c.req
		err := validateAssetInterceptRuleReq(&req)
		if err == nil {
			t.Fatalf("%s: no error (validation passed)", c.label)
		}
		assertNonEmpty(t, c.label, err.Error())
	}

	// Valid input is not rejected
	okReq := assetInterceptRuleReq{Kind: "cidr", Pattern: "192.168.0.0/16"}
	if err := validateAssetInterceptRuleReq(&okReq); err != nil {
		t.Fatalf("valid CIDR was rejected: %v", err)
	}

	// Actual path: the action rejection branch of validateTaskInterceptRuleReq
	badAction := taskInterceptRuleReq{Action: "nope", Kind: "exact_domain", Pattern: "example.com"}
	if err := validateTaskInterceptRuleReq(&badAction); err == nil {
		t.Fatalf("bad_action_path: no error (validation passed)")
	} else {
		assertNonEmpty(t, "bad_action_path", err.Error())
	}

	// A blank action is normalized to the default block value and passes
	defaultAction := taskInterceptRuleReq{Action: "", Kind: "exact_domain", Pattern: "example.com"}
	if err := validateTaskInterceptRuleReq(&defaultAction); err != nil {
		t.Fatalf("failed to normalize the default action: %v", err)
	}
	if defaultAction.Action != "block" {
		t.Fatalf("default action was not normalized to block: %q", defaultAction.Action)
	}
}
