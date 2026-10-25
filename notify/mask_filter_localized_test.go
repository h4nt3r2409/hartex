package notify

import (
	"errors"
	"strings"
	"testing"
)

// F4 ⑥: pins that the user-facing wording in notify/mask.go and filter.go (validation errors that
// server/notify_api.go returns verbatim as writeErr(400) when saving or updating notification channel settings) is English
// and contains no Han or Hangul characters. channel.go has no user-facing wording (all comments), so it is not
// a target. hasHan / hasHangul / assertEnglish are defined in notify_localized_test.go.

// TestFilterValidateLocalized checks the validation error when min_severity is given an invalid value.
// This error is exposed as 400 on the save/update paths (server/notify_api.go:264, 369).
func TestFilterValidateLocalized(t *testing.T) {
	err := Filter{MinSeverity: "hgih"}.Validate()
	if err == nil {
		t.Fatal("an invalid minimum severity must return an error")
	}
	assertEnglish(t, "Filter.Validate", err.Error())
	// low/medium/high/critical are config enums and must remain verbatim.
	for _, tok := range []string{"low", "medium", "high", "critical"} {
		if !strings.Contains(err.Error(), tok) {
			t.Errorf("severity token %q disappeared: %q", tok, err.Error())
		}
	}
	// Valid values and empty values must pass.
	if err := (Filter{MinSeverity: "high"}).Validate(); err != nil {
		t.Errorf("high is valid but returned an error: %v", err)
	}
	if err := (Filter{}).Validate(); err != nil {
		t.Errorf("an empty minimum severity is valid but returned an error: %v", err)
	}
}

// TestPrepareConfigUpdateUnknownKindLocalized checks the error when updating settings with an unregistered
// channel type (400 at server/notify_api.go:326).
func TestPrepareConfigUpdateUnknownKindLocalized(t *testing.T) {
	_, err := PrepareConfigUpdate("definitely-not-a-channel", map[string]any{}, map[string]any{})
	if err == nil {
		t.Fatal("an unregistered channel type must return an error")
	}
	assertEnglish(t, "PrepareConfigUpdate unknown kind", err.Error())
}

// TestDestinationChangedErrorLocalized triggers ErrDestinationChangedWithoutCredentials through the real path:
// a PATCH that changes only the webhook destination address (url) to a new value and makes
// no mention at all of the credentials (headers). The server returns this as 400.
func TestDestinationChangedErrorLocalized(t *testing.T) {
	_, err := PrepareConfigUpdate("webhook",
		map[string]any{
			"url":     "https://old.example.com/hook",
			"headers": map[string]any{"Authorization": "Bearer real-token"},
		},
		map[string]any{"url": "https://attacker.example.com/hook"},
	)
	if err == nil {
		t.Fatal("changing the destination without restating credentials must return an error")
	}
	var de *ErrDestinationChangedWithoutCredentials
	if !errors.As(err, &de) {
		t.Fatalf("unexpected error type: %T", err)
	}
	assertEnglish(t, "ErrDestinationChangedWithoutCredentials", err.Error())
	// The changed destination key and the missing credential key names are config field names and must appear verbatim in the message.
	if !strings.Contains(err.Error(), "url") || !strings.Contains(err.Error(), "headers") {
		t.Errorf("field key names are missing: %q", err.Error())
	}
}

// TestRejectMaskedInContainersLocalized triggers rejection of a masking sentinel inside a struct through the real path:
// a PATCH that inserts a masking sentinel value inside headers (an object and a credentials field).
func TestRejectMaskedInContainersLocalized(t *testing.T) {
	_, err := PrepareConfigUpdate("webhook",
		map[string]any{},
		map[string]any{"headers": map[string]any{"Authorization": MaskedPrefix + ":…abc123"}},
	)
	if err == nil {
		t.Fatal("a masking sentinel inside a struct must return an error")
	}
	assertEnglish(t, "rejectMaskedInContainers", err.Error())
	// The sentinel literal (__masked__) is shown in the message so operators can tell which value is the problem.
	if !strings.Contains(err.Error(), MaskedPrefix) {
		t.Errorf("the masking sentinel is missing from the message: %q", err.Error())
	}
}
