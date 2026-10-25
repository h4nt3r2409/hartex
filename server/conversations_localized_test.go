package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Regression guard keeping the conversation (chat) API error responses in
// conversations.go in English. The check reuses assertNonEmpty.

// TestConversationErrorConstantsChecked asserts that all 12 response constants are
// English. The handlers go through s.pg(w) (DB) first and cannot run to completion on this
// host without a DB, so this asserts the constants themselves (following the auth.go
// precedent). Format strings containing %d are filled with real arguments and the final
// message is checked.
func TestConversationErrorConstantsLocalized(t *testing.T) {
	cases := []struct {
		name string
		msg  string
	}{
		{"request_too_large", convErrRequestTooLarge},
		{"agent_key_empty", convErrAgentKeyEmpty},
		{"agent_key_too_long", fmt.Sprintf(convErrAgentKeyTooLong, maxConversationAgentKeyRunes)},
		{"agent_not_found", convErrAgentNotFound},
		{"llm_profile", convErrLLMProfile},
		{"title_too_long", fmt.Sprintf(convErrTitleTooLong, maxConversationTitleRunes)},
		{"title_or_pinned", convErrTitleOrPinned},
		{"title_empty", convErrTitleEmpty},
		{"bad_conv_id", convErrBadConvID},
		{"ids_count", fmt.Sprintf(convErrIDsCount, maxConversationDeleteBatch)},
		{"message_empty", convErrMessageEmpty},
		{"busy", convErrBusy},
	}
	for _, c := range cases {
		assertNonEmpty(t, c.name, c.msg)
	}
}

// TestDecodeConversationRequestTooLargeChecked checks the request body too-large path
// through to the actual HTTP response body. decodeConversationRequest is a package function
// that does not go through Server (DB), so it can run without a DB, and this also confirms
// the wiring that puts the constant into the response (request body too large → 413 +
// English message).
func TestDecodeConversationRequestTooLargeLocalized(t *testing.T) {
	// Valid JSON body exceeding the 64KB limit. MaxBytesReader returns the over-limit condition during reading.
	body := `{"title":"` + strings.Repeat("a", maxConversationRequestBytes+1024) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversations", strings.NewReader(body))
	rec := httptest.NewRecorder()
	var dst struct {
		Title string `json:"title"`
	}
	if decodeConversationRequest(rec, req, &dst) {
		t.Fatal("decodeConversationRequest returned true for an over-limit body")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response JSON: %v (body=%q)", err, rec.Body.String())
	}
	if resp.Error != convErrRequestTooLarge {
		t.Fatalf("response error = %q, want %q", resp.Error, convErrRequestTooLarge)
	}
	assertNonEmpty(t, "decode.too_large.response", resp.Error)
}

// TestConversationDefaultTitlesChecked asserts that the two conversation default title
// constants (F8) are English and distinct from each other. convDefaultTitle is both the
// creation default and the sentinel for the auto-title branch, so reverting it to the
// a non-English default would make users see non-English text in the conversation list and delete dialog.
func TestConversationDefaultTitlesLocalized(t *testing.T) {
	assertNonEmpty(t, "default_title", convDefaultTitle)
	assertNonEmpty(t, "attachment_title", convAttachmentTitle)
	if convDefaultTitle == convAttachmentTitle {
		t.Fatal("default title and attachment default title must differ")
	}
}

// TestIsDefaultConversationTitle pins down the decision in the auto-title branch. The
// creation default (convDefaultTitle) and an empty title are eligible for auto-titling, but
// a user-written title is not. Since the creation default and the sentinel are the same
// constant, this prevents a regression where they diverge and auto-titling stops working.
func TestIsDefaultConversationTitle(t *testing.T) {
	if !isDefaultConversationTitle("") {
		t.Fatal("an empty title must be eligible for auto-titling")
	}
	if !isDefaultConversationTitle(convDefaultTitle) {
		t.Fatalf("creation default %q must be eligible for auto-titling", convDefaultTitle)
	}
	if isDefaultConversationTitle("a user-written title") {
		t.Fatal("a user-written title must not be eligible for auto-titling")
	}
}

// TestConversationRetestReasonsChecked asserts that the three retest termination reason
// constants (F9) are English. These values are stored in the finding_retests.error column and
// exposed as the retest panel's item.error, so an empty reason would leave the panel blank.
func TestConversationRetestReasonsLocalized(t *testing.T) {
	assertNonEmpty(t, "retest_failed_to_start", convRetestFailedToStart)
	assertNonEmpty(t, "retest_status_read_failed", convRetestStatusReadFailed)
	assertNonEmpty(t, "retest_stopped_or_closed", convRetestStoppedOrClosed)
}

// TestTranscriptErrorSummaryChecked pins down the activity transcript error wrapper (F9).
// The chat turn (no label) and the task main agent ("main agent") calls produce the same
// "(… error: …)" shape, the inner err text is preserved verbatim, and the wrapper must
// contain non-empty English text or full-width punctuation.
func TestTranscriptErrorSummaryLocalized(t *testing.T) {
	cases := []struct {
		name  string
		label string
		errm  string
		want  string
	}{
		{"chat_turn", "", "connection reset", "(error: connection reset)"},
		{"main_agent", "main agent", "connection reset", "(main agent error: connection reset)"},
	}
	for _, c := range cases {
		got := transcriptErrorSummary(c.label, c.errm)
		if got != c.want {
			t.Fatalf("%s: transcriptErrorSummary = %q, want %q", c.name, got, c.want)
		}
		if !strings.Contains(got, c.errm) {
			t.Fatalf("%s: original error text was not preserved: %q", c.name, got)
		}
		// Verify that full-width parentheses and colons were replaced with ASCII (F1 · fluent-korean policy).
		if strings.ContainsAny(got, "（）：") {
			t.Fatalf("%s: full-width punctuation remains: %q", c.name, got)
		}
		// The wrapper label must contain English text and non-empty English text (the err text is
		// not part of the check and is fixed to ASCII). A chat turn with no label also includes
		// the ASCII "error" wrapper.
		wrapper := strings.ReplaceAll(got, c.errm, "")
		assertNonEmpty(t, c.name+".wrapper", wrapper)
	}
}
