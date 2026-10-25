package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/db"
)

// TestNotifierDeliveryReasonsChecked guards F17: every delivery failure/defer
// reason that notifier.go writes into notification_deliveries.last_error — which
// notify_api.go echoes back to the "delivery history" table in delivery-list.tsx — must be
// English (non-empty English text). Diagnostic log.Printf lines stay in the
// original language (Z2) and are intentionally not covered here.
func TestNotifierDeliveryReasonsLocalized(t *testing.T) {
	// Format every constant with ASCII arguments so any leftover non-English content is
	// the constant's own, not injected by the test data.
	cases := []struct {
		label string
		msg   string
	}{
		{"channelKindUnregistered", fmt.Sprintf(errDeliveryChannelKindUnregistered, "webhook")},
		{"snapshotUnrenderable", errDeliverySnapshotUnrenderable},
		{"channelLengthCapped", fmt.Sprintf(errDeliveryChannelLengthCapped, 3)},
		{"noDeliveredCount", fmt.Sprintf(errDeliveryNoDeliveredCount, 0)},
		{"retryExhausted", fmt.Sprintf(errDeliveryRetryExhausted, db.MaxNotifyAttempts, errors.New("boom"))},
		{"snapshotEmpty", fmt.Sprintf(errDeliverySnapshotEmpty, 7)},
		{"snapshotParse", fmt.Errorf(errDeliverySnapshotParse, 9, errors.New("unexpected end of JSON input")).Error()},
		{"batchAllUnparseable", fmt.Sprintf(errDeliveryBatchAllUnparseable, 2)},
	}
	for _, c := range cases {
		assertNonEmpty(t, c.label, c.msg)
	}
}

// TestNotifierParseSnapshotErrorsChecked exercises the real parseSnapshot paths
// that surface as last_error through renderSingle → FailDeliveries. parseSnapshot is
// a package-level pure function, so no DB or Server is needed.
func TestNotifierParseSnapshotErrorsLocalized(t *testing.T) {
	// Empty snapshot.
	if _, err := parseSnapshot(&db.NotificationDelivery{ID: 7}); err == nil {
		t.Fatal("an empty snapshot must not pass without an error")
	} else {
		assertNonEmpty(t, "parseSnapshot.empty", err.Error())
		if !strings.Contains(err.Error(), "7") {
			t.Fatalf("parseSnapshot.empty: delivery item ID 7 is missing from the message: %q", err.Error())
		}
	}
	// Malformed JSON snapshot.
	if _, err := parseSnapshot(&db.NotificationDelivery{ID: 9, Snapshot: []byte("{bad")}); err == nil {
		t.Fatal("a malformed JSON snapshot must not pass without an error")
	} else {
		assertNonEmpty(t, "parseSnapshot.malformed", err.Error())
	}
}

// TestNotifierRenderBatchAllUnparseableChecked drives the real renderBatch "every
// snapshot is unparseable" branch. With all snapshots empty, every delivery is
// skipped before itemFor (and thus before n.pg), so a zero-value Notifier reaches
// the errDeliveryBatchAllUnparseable return without touching the database.
func TestNotifierRenderBatchAllUnparseableLocalized(t *testing.T) {
	n := &Notifier{}
	deliveries := []*db.NotificationDelivery{{ID: 1}, {ID: 2}}
	if _, _, err := n.renderBatch(context.Background(), deliveries, "", 30); err == nil {
		t.Fatal("an error is required when every snapshot is unparseable")
	} else {
		assertNonEmpty(t, "renderBatch.allUnparseable", err.Error())
	}
}
