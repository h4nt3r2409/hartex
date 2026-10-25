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
		t.Fatalf("카드 직렬화 실패: %v", err)
	}
	return string(b)
}

// TestFeishuItemLinesLocalized checks that the field labels in a single-item Feishu card body (status change, type,
// asset, summary) are Korean and identical character-for-character to markdown.go (F4②). feishu.go uses
// its own copy of these labels (not the shared markdown 3-channel function), so besides the shared-function regression,
// this pin is needed to catch a regression where only the Feishu body reverts to Chinese.
func TestFeishuItemLinesLocalized(t *testing.T) {
	got := feishuItemLines(asciiStatusChangeItem())
	assertKorean(t, "feishuItemLines", got)
	for _, want := range []string{
		"**Status change**: Pending → Fixed",
		"**Type**: SQLi",
		"**Assets**: a.example.com",
		"**Summary**: SQL injection via q param",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("飞书 단건 본문에 %q 가 있어야 합니다:\n%s", want, got)
		}
	}
}

// TestFeishuCardButtonsLocalized checks that the Feishu card button labels (view details, view all on platform) are
// Korean, and that the entire card contains no Chinese Han characters (with ASCII data).
func TestFeishuCardButtonsLocalized(t *testing.T) {
	// Single item: "상세 보기" button (when DetailURL is present)
	single, _ := feishuCard(Message{Items: []Item{asciiStatusChangeItem()}})
	singleJSON := mustJSON(t, single)
	if hasHan(singleJSON) {
		t.Errorf("단건 飞书 카드에 중국어 한자가 남았습니다:\n%s", singleJSON)
	}
	if !strings.Contains(singleJSON, "View details") {
		t.Errorf("단건 飞书 카드에 'View details' 버튼이 있어야 합니다:\n%s", singleJSON)
	}

	// Batch: "플랫폼에서 전체 보기" button (when HomeURL is present)
	batch, _ := feishuCard(Message{Batch: true, HomeURL: "https://platform.example", Items: hanFreeItems(2)})
	batchJSON := mustJSON(t, batch)
	if hasHan(batchJSON) {
		t.Errorf("다건 飞书 카드에 중국어 한자가 남았습니다:\n%s", batchJSON)
	}
	if !strings.Contains(batchJSON, "View all on the platform") {
		t.Errorf("다건 飞书 카드에 'View all on the platform' 버튼이 있어야 합니다:\n%s", batchJSON)
	}
}

// TestDingTalkActionCardButtonLocalized checks via the real Send path (fake receiver) that the DingTalk ActionCard's "상세 보기" button (singleTitle)
// is Korean.
func TestDingTalkActionCardButtonLocalized(t *testing.T) {
	var singleTitle string
	srv := capturePost(t, `{"errcode":0,"errmsg":"ok"}`, func(_ *testing.T, body map[string]any, _ *http.Request) {
		card, _ := body["actionCard"].(map[string]any)
		singleTitle, _ = card["singleTitle"].(string)
	})
	m := Message{Items: []Item{asciiStatusChangeItem()}}
	if _, err := (dingTalkChannel{}).Send(context.Background(), map[string]any{"webhook": srv.URL}, m); err != nil {
		t.Fatalf("투递 실패: %v", err)
	}
	if singleTitle != "View details" {
		t.Errorf("钉钉 ActionCard singleTitle 은 'View details' 여야 합니다, 받은 값 %q", singleTitle)
	}
}

// TestChinaPlatformValidateLocalized checks that DingTalk, Feishu, and WeCom configuration validation errors are Korean and
// keep the "Webhook" wording (exposed to users by the notification settings save/test API).
func TestChinaPlatformValidateLocalized(t *testing.T) {
	for _, kind := range []string{KindDingTalk, KindFeishu, KindWeCom} {
		ch, ok := Get(kind)
		if !ok {
			t.Fatalf("%s 채널이 등록되어 있지 않습니다", kind)
		}

		// Empty config: "Webhook 주소가 없습니다"
		missing := ch.Validate(map[string]any{})
		if missing == nil {
			t.Fatalf("%s: 빈 설정은 검증에 실패해야 합니다", kind)
		}
		assertKorean(t, kind+" missing", missing.Error())
		if !strings.Contains(missing.Error(), "Webhook") || !strings.Contains(missing.Error(), "missing") {
			t.Errorf("%s: 누락 오류는 'Webhook address is missing' 여야 합니다, 받은 값 %q", kind, missing)
		}

		// Invalid address (ftp): "Webhook 주소가 올바르지 않습니다: ..."
		bad := ch.Validate(map[string]any{"webhook": "ftp://x"})
		if bad == nil {
			t.Fatalf("%s: ftp 주소는 검증에 실패해야 합니다", kind)
		}
		if hasHan(bad.Error()) {
			t.Errorf("%s: 잘못된 주소 오류에 중국어 한자가 남았습니다: %q", kind, bad)
		}
		if !strings.Contains(bad.Error(), "invalid Webhook address") {
			t.Errorf("%s: 잘못된 주소 오류는 'invalid Webhook address' 로 시작해야 합니다, 받은 값 %q", kind, bad)
		}
	}
}

// TestChinaPlatformSendErrorsLocalized checks that send-failure reasons for DingTalk, Feishu, and WeCom are recorded in Korean
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
			t.Fatalf("%s 채널이 등록되어 있지 않습니다", tc.kind)
		}
		_, err := ch.Send(context.Background(), map[string]any{"webhook": srv.URL}, singleMsg())
		if err == nil {
			t.Fatalf("%s: %s 응답은 오류여야 합니다", tc.kind, tc.resp)
		}
		if hasHan(err.Error()) {
			t.Errorf("%s: 발송 오류에 중국어 한자가 남았습니다: %q", tc.kind, err)
		}
		for _, sub := range tc.wantSubs {
			if !strings.Contains(err.Error(), sub) {
				t.Errorf("%s: 발송 오류에 %q 가 있어야 합니다, 받은 값 %q", tc.kind, sub, err)
			}
		}
	}

	// Parse-failure path when the response is not JSON: "<플랫폼> 응답을 해석하지 못했습니다"
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
			t.Fatalf("%s 채널이 등록되어 있지 않습니다", pc.kind)
		}
		_, err := ch.Send(context.Background(), map[string]any{"webhook": srv.URL}, singleMsg())
		if err == nil {
			t.Fatalf("%s: JSON 이 아닌 응답은 오류여야 합니다", pc.kind)
		}
		want := "failed to parse " + pc.platform + " response"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: 파싱 실패 오류에 %q 가 있어야 합니다, 받은 값 %q", pc.kind, want, err)
		}
	}
}
