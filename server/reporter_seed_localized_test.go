package server

import "testing"

// TestReporterSeedLabelsLocalized is a regression guard that keeps the display-only labels
// of the seeded reporter agent (reporterAgentName, reporterAgentDescription) in Korean.
// These two strings are rendered only through `agentDTO` (server_mgmt.go) into the
// system/agents UI, and are pure UI labels that enter neither any agent system prompt
// (renderSystem assembles only key→section[A] agent.ReporterDefaultPrompt) nor model-based
// agent selection (the reporter trigger is report_finding tool-call based OnToolCall and
// deterministic). So localizing them to Korean does not touch BRIEF boundary #1 (no brain
// translation). Conversely, the reporter's brain body (agent.ReporterDefaultPrompt) and the
// trigger injection message (reporterToolCallMessage) are model input and preserve the
// original Chinese, so they are not the target of this test. The Korean check reuses
// assertKoreanError (contains Hangul, zero Chinese characters,
// intercept_archive_localized_test.go). [[F35]] [[G133]]
func TestReporterSeedLabelsLocalized(t *testing.T) {
	assertKoreanError(t, "reporter_name", reporterAgentName)
	assertKoreanError(t, "reporter_description", reporterAgentDescription)
}
