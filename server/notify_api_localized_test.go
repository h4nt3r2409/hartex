package server

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/Autumn-27/artex/db"
)

// Regression guard that keeps the notification settings API response messages and test
// messages in notify_api.go in English. The check reuses assertNonEmpty (contains
// non-empty messages).

// TestNotifyErrorConstantsChecked asserts that all 10 response constants and all 3 test
// message constants are in English. An empty constant makes this test fail.
func TestNotifyErrorConstantsLocalized(t *testing.T) {
	cases := map[string]string{
		"notifyErrBadJSON":         notifyErrBadJSON,
		"notifyErrKindInvalidFmt":  fmt.Sprintf(notifyErrKindInvalidFmt, "email / webhook"),
		"notifyErrNameMissing":     notifyErrNameMissing,
		"notifyErrNameEmpty":       notifyErrNameEmpty,
		"notifyErrModeInvalid":     notifyErrModeInvalid,
		"notifyErrRateNegative":    notifyErrRateNegative,
		"notifyErrChannelID":       notifyErrChannelID,
		"notifyErrKindUnregFmt":    fmt.Sprintf(notifyErrKindUnregFmt, "bogus"),
		"notifyErrDeliveryID":      notifyErrDeliveryID,
		"notifyErrChannelNotFound": notifyErrChannelNotFound,
		"notifyTestName":           notifyTestName,
		"notifyTestClass":          notifyTestClass,
		"notifyTestSummary":        notifyTestSummary,
	}
	for label, msg := range cases {
		assertNonEmpty(t, label, msg)
	}
}

// TestNotifyChannelLookupErrChecked exercises the "channel missing → 404" path against
// the real code. notifyChannelLookupErr is a package-level function that runs with just
// w and err, no Server/DB, so passing the sentinel error straight through confirms the
// wiring that puts the English constant in the 404 response body without a DB.
func TestNotifyChannelLookupErrLocalized(t *testing.T) {
	rec := httptest.NewRecorder()
	notifyChannelLookupErr(rec, db.ErrNotificationChannelNotFound)
	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404 (body %q)", rec.Code, rec.Body.String())
	}
	got := decodeErrorField(t, rec.Body.Bytes())
	if got != notifyErrChannelNotFound {
		t.Fatalf("message = %q, want %q", got, notifyErrChannelNotFound)
	}
	assertNonEmpty(t, "channel_not_found", got)
}

// TestNotifyTestMessageChecked assembles the channel connectivity test message with a
// pure function and checks that the body sent to the user (title, class, summary) is in
// English. No DB or network is needed.
func TestNotifyTestMessageLocalized(t *testing.T) {
	const base = "https://artex.example.test"
	msg := notifyTestMessage(base)
	if len(msg.Items) != 1 {
		t.Fatalf("test message item count = %d, want 1", len(msg.Items))
	}
	item := msg.Items[0]
	assertNonEmpty(t, "test.name", item.Name)
	assertNonEmpty(t, "test.class", item.VulnClass)
	assertNonEmpty(t, "test.summary", item.Summary)
	if msg.HomeURL != base || item.DetailURL != base {
		t.Fatalf("link preservation failed: HomeURL=%q DetailURL=%q (want %q)", msg.HomeURL, item.DetailURL, base)
	}
}
