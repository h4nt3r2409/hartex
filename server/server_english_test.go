package server

import "testing"

func TestFallbackCommandRejectsNonEnglishAliases(t *testing.T) {
	if cmd, _ := fallbackCommand("意图: 读取报告"); cmd == "intent" {
		t.Fatal("non-English intent alias must no longer be accepted")
	}
	if cmd, text := fallbackCommand("intent: read the report"); cmd != "intent" || text != "read the report" {
		t.Fatalf("English intent alias = %q, %q", cmd, text)
	}
}
