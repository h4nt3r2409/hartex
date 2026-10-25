package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// asciiStatusChangeItem builds a status-change item with no Han characters (ASCII only).
// If a label regresses to Chinese, Han characters appear in the rendered result, so only when the data has zero Han characters
// does the "zero Han characters in the entire output" assertion precisely catch label regressions.
func asciiStatusChangeItem() Item {
	return Item{
		FindingID:  7,
		Name:       "login-flaw",
		VulnClass:  "SQLi",
		Severity:   "high",
		Summary:    "SQL injection via q param",
		Assets:     []string{"a.example.com"},
		DetailURL:  "https://platform.example/finding/7",
		FromStatus: "pending",
		ToStatus:   "fixed",
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to serialize the card: %v", err)
	}
	return string(b)
}

// TestFeishuItemLinesLocalized checks that the field labels in a single-item Feishu card body (status change, type,
// asset, summary) are English-only and identical character-for-character to markdown.go (F4②). feishu.go uses
// its own copy of these labels (not the shared markdown 3-channel function), so besides the shared-function regression,
// this pin is needed to catch a regression where only the Feishu body reverts to Chinese.
func TestFeishuItemLinesLocalized(t *testing.T) {
	got := feishuItemLines(asciiStatusChangeItem())
	assertEnglish(t, "feishuItemLines", got)
	for _, want := range []string{
		"**Status change**: Pending → Fixed",
		"**Type**: SQLi",
		"**Assets**: a.example.com",
		"**Summary**: SQL injection via q param",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Feishu single-item body must contain %q:\n%s", want, got)
		}
	}
}

// TestFeishuCardButtonsLocalized checks that the Feishu card button labels (view details, view all on platform) are
// English-only, and that the entire card contains no Chinese Han characters (with ASCII data).
func TestFeishuCardButtonsLocalized(t *testing.T) {
	// Single item: "View details" button (when DetailURL is present)
	single, _ := feishuCard(Message{Items: []Item{asciiStatusChangeItem()}})
	singleJSON := mustJSON(t, single)
	if hasHan(singleJSON) {
		t.Errorf("Han characters remain in the single-item Feishu card:\n%s", singleJSON)
	}
	if !strings.Contains(singleJSON, "View details") {
		t.Errorf("the single-item Feishu card must have a 'View details' button:\n%s", singleJSON)
	}

	// Batch: "View all on the platform" button (when HomeURL is present)
	batch, _ := feishuCard(Message{Batch: true, HomeURL: "https://platform.example", Items: hanFreeItems(2)})
	batchJSON := mustJSON(t, batch)
	if hasHan(batchJSON) {
		t.Errorf("Han characters remain in the batch Feishu card:\n%s", batchJSON)
	}
	if !strings.Contains(batchJSON, "View all on the platform") {
		t.Errorf("the batch Feishu card must have a 'View all on the platform' button:\n%s", batchJSON)
	}
}

// TestDingTalkActionCardButtonLocalized checks via the real Send path (fake receiver) that the DingTalk ActionCard's "View details" button (singleTitle)
// is English.
func TestDingTalkActionCardButtonLocalized(t *testing.T) {
	var singleTitle string
	srv := capturePost(t, `{"errcode":0,"errmsg":"ok"}`, func(_ *testing.T, body map[string]any, _ *http.Request) {
		card, _ := body["actionCard"].(map[string]any)
		singleTitle, _ = card["singleTitle"].(string)
	})
	m := Message{Items: []Item{asciiStatusChangeItem()}}
	if _, err := (dingTalkChannel{}).Send(context.Background(), map[string]any{"webhook": srv.URL}, m); err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if singleTitle != "View details" {
		t.Errorf("DingTalk ActionCard singleTitle must be 'View details', got %q", singleTitle)
	}
}

// TestChinaPlatformValidateLocalized checks that DingTalk, Feishu, and WeCom configuration validation errors are English and
// keep the "Webhook" wording (exposed to users by the notification settings save/test API).
func TestChinaPlatformValidateLocalized(t *testing.T) {
	for _, kind := range []string{KindDingTalk, KindFeishu, KindWeCom} {
		ch, ok := Get(kind)
		if !ok {
			t.Fatalf("channel %s is not registered", kind)
		}

		// Empty config: "Webhook address is missing"
		missing := ch.Validate(map[string]any{})
		if missing == nil {
			t.Fatalf("%s: an empty config must fail validation", kind)
		}
		assertEnglish(t, kind+" missing", missing.Error())
		if !strings.Contains(missing.Error(), "Webhook") || !strings.Contains(missing.Error(), "missing") {
			t.Errorf("%s: the missing-field error must be 'Webhook address is missing', got %q", kind, missing)
		}

		// Invalid address (ftp): "invalid Webhook address: ..."
		bad := ch.Validate(map[string]any{"webhook": "ftp://x"})
		if bad == nil {
			t.Fatalf("%s: an ftp address must fail validation", kind)
		}
		if hasHan(bad.Error()) {
			t.Errorf("%s: Han characters remain in the bad-address error: %q", kind, bad)
		}
		if !strings.Contains(bad.Error(), "invalid Webhook address") {
			t.Errorf("%s: the bad-address error must start with 'invalid Webhook address', got %q", kind, bad)
		}
	}
}

// TestChinaPlatformSendErrorsLocalized checks that send-failure reasons for DingTalk, Feishu, and WeCom are recorded in English
// in the delivery history (last_error). Platform names match the UI channel labels (DingTalk/Feishu/
// WeCom). Server errmsg is ASCII, so if Han characters appear in an error, the skeleton wording has
// regressed to Chinese.
func TestChinaPlatformSendErrorsLocalized(t *testing.T) {
	// Business error code path (HTTP 200 + body errcode/code != 0).
	bizCases := []struct {
		kind     string
		resp     string
		wantSubs []string
	}{
		{KindDingTalk, `{"errcode":310000,"errmsg":"keyword not matched"}`, []string{"DingTalk returned an error", "310000"}},
		{KindFeishu, `{"code":19021,"msg":"sign error"}`, []string{"Feishu returned an error", "19021"}},
		{KindWeCom, `{"errcode":45009,"errmsg":"freq out of limit"}`, []string{"WeCom request was rate limited", "45009"}},
		{KindWeCom, `{"errcode":93000,"errmsg":"invalid webhook"}`, []string{"WeCom returned an error", "93000"}},
	}
	for _, tc := range bizCases {
		srv := capturePost(t, tc.resp, nil)
		ch, ok := Get(tc.kind)
		if !ok {
			t.Fatalf("channel %s is not registered", tc.kind)
		}
		_, err := ch.Send(context.Background(), map[string]any{"webhook": srv.URL}, singleMsg())
		if err == nil {
			t.Fatalf("%s: response %s must be an error", tc.kind, tc.resp)
		}
		if hasHan(err.Error()) {
			t.Errorf("%s: Han characters remain in the send error: %q", tc.kind, err)
		}
		for _, sub := range tc.wantSubs {
			if !strings.Contains(err.Error(), sub) {
				t.Errorf("%s: the send error must contain %q, got %q", tc.kind, sub, err)
			}
		}
	}

	// Parse-failure path when the response is not JSON: "failed to parse <platform> response"
	parseCases := []struct {
		kind     string
		platform string
	}{
		{KindDingTalk, "DingTalk"},
		{KindFeishu, "Feishu"},
		{KindWeCom, "WeCom"},
	}
	for _, pc := range parseCases {
		srv := capturePost(t, `not-json`, nil)
		ch, ok := Get(pc.kind)
		if !ok {
			t.Fatalf("channel %s is not registered", pc.kind)
		}
		_, err := ch.Send(context.Background(), map[string]any{"webhook": srv.URL}, singleMsg())
		if err == nil {
			t.Fatalf("%s: a non-JSON response must be an error", pc.kind)
		}
		want := "failed to parse " + pc.platform + " response"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: the parse-failure error must contain %q, got %q", pc.kind, want, err)
		}
	}
}
