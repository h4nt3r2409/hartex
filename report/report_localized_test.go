package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/Autumn-27/artex/db"
)

// containsNonEnglish returns the first Han or Hangul character found in s, if any.
// English-only output must never contain one.
func containsNonEnglish(s string) (rune, bool) {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) {
			return r, true
		}
	}
	return 0, false
}

func sampleFindingNode(t *testing.T) *db.Node {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"vulnclass": "SQL Injection",
		"name":      "Login form SQL injection",
		"severity":  "high",
		"summary":   "Error-based SQL injection was confirmed in the login parameter.",
		"evidence":  map[string]string{"poc": "' OR '1'='1"},
	})
	if err != nil {
		t.Fatalf("failed to serialize payload: %v", err)
	}
	return &db.Node{ID: 1, Kind: "finding", Payload: payload}
}

func sampleDBFinding() *db.DBFinding {
	return &db.DBFinding{
		ID:              123,
		VulnClass:       "SQL Injection",
		Name:            "Login form SQL injection",
		Severity:        "high",
		Summary:         "Error-based SQL injection was confirmed in the login parameter.",
		Evidence:        "' OR '1'='1",
		Status:          "confirmed",
		Report:          "Detailed analysis body.",
		CreatedAt:       time.Date(2026, 10, 4, 2, 30, 0, 0, time.UTC),
		EvidenceVersion: 2,
		TaskDescription: "Demo target penetration test",
		TrafficBindings: []db.FindingTrafficBinding{
			{
				ID:   9,
				Role: "request",
				Note: "injection payload submission point",
				Snapshot: db.TrafficEvidenceSnapshot{
					Method: "POST",
					URL:    "https://sandbox.local/login",
					Status: 200,
				},
			},
		},
	}
}

// TestReportMarkdownLocalized verifies that the skeleton of the exploration-graph report
// (report.Markdown) is English-only and contains no Han or Hangul characters.
func TestReportMarkdownLocalized(t *testing.T) {
	out := Markdown(Input{
		Title:       "demo task",
		Goal:        "full sandbox review",
		GeneratedAt: time.Date(2026, 10, 4, 2, 30, 0, 0, time.UTC),
		AssetCounts: map[string]int{"host": 2, "url": 5},
		Findings:    []*db.Node{sampleFindingNode(t)},
	})
	if r, ok := containsNonEnglish(out); ok {
		t.Fatalf("non-English character %q remains in the report skeleton:\n%s", string(r), out)
	}
	for _, want := range []string{
		"# Penetration Test Report: demo task",
		"- **Task goal**: full sandbox review",
		"- **Generated at**:",
		"## Summary",
		"- Confirmed findings: **1**",
		"## Findings",
		"**PoC / Evidence:**",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report skeleton is missing %q", want)
		}
	}
}

// TestReportMarkdownEmptyLocalized verifies that the guidance text shown when there are no findings is English.
func TestReportMarkdownEmptyLocalized(t *testing.T) {
	out := Markdown(Input{Title: "empty task", GeneratedAt: time.Now()})
	if r, ok := containsNonEnglish(out); ok {
		t.Fatalf("non-English character %q remains in the empty report:\n%s", string(r), out)
	}
	if !strings.Contains(out, "_No findings were confirmed in this run._") {
		t.Errorf("empty-report guidance is not English:\n%s", out)
	}
}

// TestFindingsMarkdownLocalized verifies that the skeleton of the finding summary/detail
// Markdown export and the severity labels are English-only and contain no Han or Hangul characters.
func TestFindingsMarkdownLocalized(t *testing.T) {
	f := sampleDBFinding()
	out := FindingsMarkdown([]*db.DBFinding{f}, time.Date(2026, 10, 4, 2, 30, 0, 0, time.UTC))
	if r, ok := containsNonEnglish(out); ok {
		t.Fatalf("non-English character %q remains in the findings summary report:\n%s", string(r), out)
	}
	for _, want := range []string{
		"# Findings Summary Report",
		"- **Generated at**:",
		"- **Total findings**: 1",
		"## Summary",
		"| Severity | Count |",
		"| Critical |", "| High |", "| Medium |", "| Low |", // severity labels (same as UI status.severity)
		"## Finding Details",
		"- **Class**: SQL Injection",
		"- **Status**: confirmed",
		"- **Task**: Demo target penetration test",
		"- **Found at**:",
		"**Evidence:**",
		"**Detailed report:**",
		"## Related Traffic Evidence",
		"Evidence version: 2, binding count: 1.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary report is missing %q", want)
		}
	}

	single := SingleFindingMarkdown(f, time.Date(2026, 10, 4, 2, 30, 0, 0, time.UTC))
	if r, ok := containsNonEnglish(single); ok {
		t.Fatalf("non-English character %q remains in the single-finding report:\n%s", string(r), single)
	}
	for _, want := range []string{
		"- **Severity**: high",
		"## Overview",
		"## Evidence",
		"## Detailed report",
		"[Request message](evidence/123/9/request.http)",
		"[Response message](evidence/123/9/response.http)",
	} {
		if !strings.Contains(single, want) {
			t.Errorf("single-finding report is missing %q", want)
		}
	}
}

// TestFindingsCSVLocalized verifies that the CSV export header is English-only and contains no Han or Hangul characters.
// (Only the header row after the BOM needs to be English. Data values are left as-is.)
func TestFindingsCSVLocalized(t *testing.T) {
	raw := FindingsCSV([]*db.DBFinding{sampleDBFinding()})
	out := strings.TrimPrefix(string(raw), "\xEF\xBB\xBF")
	header := strings.SplitN(out, "\n", 2)[0]
	if r, ok := containsNonEnglish(header); ok {
		t.Fatalf("non-English character %q remains in the CSV header: %s", string(r), header)
	}
	for _, col := range []string{"ID", "Name", "Class", "Severity", "Status", "Task", "Found at", "Overview", "Traffic evidence count", "Traffic evidence IDs"} {
		if !strings.Contains(header, col) {
			t.Errorf("CSV header is missing the %q column: %s", col, header)
		}
	}
}
