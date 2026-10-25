package sidequestion

import (
	"strings"
	"testing"
)

func TestInstructionRequiresEnglishAnswer(t *testing.T) {
	if !strings.Contains(instruction, "English") {
		t.Fatalf("side-question instruction must require English: %q", instruction)
	}
}
