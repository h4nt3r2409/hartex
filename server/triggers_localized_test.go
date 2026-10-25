package server

import "testing"

// Regression test that keeps triggers.go's trigger configuration validation error messages in
// English. The check reuses assertNonEmpty (non-empty English text).

// TestTriggerErrorConstantsChecked asserts that all 3 error constants are English.
func TestTriggerErrorConstantsLocalized(t *testing.T) {
	cases := []struct {
		name string
		msg  string
	}{
		{"no_condition", errTriggerNoCondition},
		{"tool_set_empty", errTriggerToolSetEmpty},
		{"custom_only", errTriggerCustomOnly},
	}
	for _, c := range cases {
		assertNonEmpty(t, c.name, c.msg)
	}
}

// TestValidateTriggerChecked actually calls the pure validateTrigger function, confirming
// that the missing-condition and no-tool-selected paths return English constants and that valid
// input returns an empty string. These return values are surfaced to users as-is by
// pgCreateTrigger/pgUpdateTrigger via writeErr(400, msg).
func TestValidateTriggerLocalized(t *testing.T) {
	if msg := validateTrigger(&triggerReq{}); msg != errTriggerNoCondition {
		t.Fatalf("missing-condition message = %q, want %q", msg, errTriggerNoCondition)
	}
	if msg := validateTrigger(&triggerReq{OnToolCall: true}); msg != errTriggerToolSetEmpty {
		t.Fatalf("no-tool message = %q, want %q", msg, errTriggerToolSetEmpty)
	}
	if msg := validateTrigger(&triggerReq{OnFinding: true}); msg != "" {
		t.Fatalf("valid-input message = %q, want an empty string", msg)
	}
	if msg := validateTrigger(&triggerReq{OnToolCall: true, ToolNames: []string{"nmap"}}); msg != "" {
		t.Fatalf("valid tool-call message = %q, want an empty string", msg)
	}
}
