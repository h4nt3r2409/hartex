package server

import (
	"context"
	"errors"
	"testing"
)

// TestControlWorkNoRunningWorkError drives ControlWork without a DB or engine to
// verify that it returns an error when there is no running work, and that the
// errWorkControlConflict sentinel remains wrapped for errors.Is.
func TestControlWorkNoRunningWorkError(t *testing.T) {
	e := NewEngine(nil)
	err := e.ControlWork(context.Background(), 42, "pause")
	if err == nil {
		t.Fatal("ControlWork returned nil with no running work")
	}
	if !errors.Is(err, errWorkControlConflict) {
		t.Fatalf("errors.Is(err, errWorkControlConflict) = false, err=%v", err)
	}
}
