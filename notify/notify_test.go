package notify

import "testing"

func TestLabelsAreEnglish(t *testing.T) {
	if got := SeverityLabel("critical"); got != "🔴 Critical" {
		t.Fatalf("SeverityLabel(critical) = %q", got)
	}
	if got := StatusLabel("confirmed"); got != "Confirmed" {
		t.Fatalf("StatusLabel(confirmed) = %q", got)
	}
}
