package intercept

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJudgeActionLabelIsEnglish(t *testing.T) {
	if got := judgeActionLabel("deny"); got != "deny" {
		t.Fatalf("judgeActionLabel(deny) = %q", got)
	}
	for _, action := range []string{"allow", "ask"} {
		if got := judgeActionLabel(action); got != action {
			t.Fatalf("judgeActionLabel(%s) = %q", action, got)
		}
	}
	if got := judgeActionLabel("weird"); got != "weird" {
		t.Fatalf("judgeActionLabel(weird) = %q, want passthrough", got)
	}
}

func TestJudgePromptRequiresEnglishComment(t *testing.T) {
	if !strings.Contains(DefaultJudgePrompt, "Write the comment value in English") {
		t.Fatalf("judge prompt must require an English comment")
	}
}

func TestParseVerdict(t *testing.T) {
	for _, action := range []string{"allow", "ask", "deny"} {
		t.Run(action, func(t *testing.T) {
			reason := "Actual operation: write a report containing the words ALLOW, DENY, and ASK; Consequence if successful: saves the text, and the commands in the body do not execute; Matched rule: custom clause"
			raw, _ := json.Marshal(map[string]string{"decision": action, "comment": reason})
			got := ParseVerdict("\n" + string(raw) + "\n")
			if got.Action != action || got.Reason != reason {
				t.Fatalf("lost verdict or explanation: %+v", got)
			}
		})
	}
}

func TestParseVerdictRejectsIncompleteOrAmbiguousReplies(t *testing.T) {
	valid := `{"decision":"allow","comment":"Actual operation: read the file; Consequence if successful: returns the contents; Matched rule: A5"}`
	for _, reply := range []string{
		"", "ALLOW", "DENY:hits D4", "allow:ALLOW", "ASK:ownership unclear",
		`{"decision":"allow"}`, `{"decision":"approve","comment":"Actual operation: read; Consequence if successful: returns the contents; Matched rule: A5"}`,
		`{"decision":"allow","comment":null}`, `{"decision":"allow","comment":123}`,
		strings.Replace(valid, "Actual operation: read the file", "Actual operation: ", 1),
		strings.Replace(valid, "Consequence if successful: returns the contents", "Consequence if successful: ", 1),
		strings.Replace(valid, "Matched rule: A5", "Matched rule: ", 1),
		strings.Replace(valid, "; Matched rule: A5", "", 1),
		// English prefix and Chinese separators (or vice versa) must not mix:
		// the comment has to use one complete marker set.
		strings.Replace(valid, "; Consequence if successful: ", "；成功后的后果：", 1),
		strings.Replace(valid, "; Matched rule: A5", "；命中规则：A5", 1),
		strings.Replace(valid, `"decision":"allow"`, `"decision":"deny","decision":"allow"`, 1),
		strings.Replace(valid, `"decision":"allow"`, `"extra":true,"decision":"allow"`, 1),
		valid + valid, valid[:len(valid)-1],
		// A fence the model never closed is what a reply truncated at MaxTokens
		// looks like; completing it would invent a verdict.
		"```json\n" + valid[:len(valid)-1],
		"```json\n" + valid + "\n```\nI also recommend a follow-up manual review.",
		"My verdict is:\n" + valid,
	} {
		if got := ParseVerdict(reply); got.Action != "" {
			t.Errorf("accepted incomplete/ambiguous verdict: %q => %+v", reply, got)
		}
	}
}

// Wrapping JSON in markdown is the one deviation models make routinely. Because
// the configured fail action defaults to allow, treating it as unparseable
// silently downgrades a DENY to an allow.
func TestParseVerdictUnwrapsCodeFence(t *testing.T) {
	deny := `{"decision":"deny","comment":"Actual operation: delete production files; Consequence if successful: business data is lost; Matched rule: D4"}`
	for _, reply := range []string{
		"```json\n" + deny + "\n```",
		"```JSON\n" + deny + "\n```",
		"```\n" + deny + "\n```",
		"  ```json\n" + deny + "\n```  ",
	} {
		got := ParseVerdict(reply)
		if got.Action != "deny" || !strings.HasSuffix(got.Reason, "Matched rule: D4") {
			t.Errorf("fenced verdict lost: %q => %+v", reply, got)
		}
	}
}

func TestParseVerdictKeepsCompleteExplanation(t *testing.T) {
	reason := "Actual operation: " + strings.Repeat("write report", 30) + "; Consequence if successful: only saves the file; Matched rule: A2"
	raw, _ := json.Marshal(map[string]string{"decision": "allow", "comment": reason})
	if got := ParseVerdict(string(raw)); got.Reason != reason {
		t.Fatal("explanation was truncated or lost its rule")
	}
	raw, _ = json.Marshal(map[string]string{"decision": "allow", "comment": strings.Repeat("x", 2401)})
	if got := ParseVerdict(string(raw)); got.Action != "" {
		t.Fatal("accepted unbounded explanation")
	}
}

// TestParseVerdictLegacyChineseMarkers covers backward compatibility only:
// verdicts persisted or emitted by prompts predating the English contract must
// still parse. New output always uses the English markers.
func TestParseVerdictLegacyChineseMarkers(t *testing.T) {
	reason := "实际操作：删除生产文件；成功后的后果：业务数据丢失；命中规则：D4"
	raw, _ := json.Marshal(map[string]string{"decision": "deny", "comment": reason})
	got := ParseVerdict(string(raw))
	if got.Action != "deny" || got.Reason != reason {
		t.Fatalf("legacy Chinese verdict lost: %+v", got)
	}
}
