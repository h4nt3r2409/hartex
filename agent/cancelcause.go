package agent

import (
	"context"
	"errors"
	"fmt"
)

// AbortCause names why an agent run's context was cancelled. Every cancellation
// site should attach one so the activity trace can report the real initiator.
type AbortCause struct {
	Code  string
	Short string
	Text  string
}

func (c *AbortCause) Error() string { return c.Text }

func cause(code, short, text string) *AbortCause {
	return &AbortCause{Code: code, Short: short, Text: text}
}

// Causef builds a cause that includes runtime-specific detail.
func Causef(code, short, format string, args ...any) *AbortCause {
	return &AbortCause{Code: code, Short: short, Text: fmt.Sprintf(format, args...)}
}

var (
	// Task-level execution context.
	AbortPausedByUser = cause("paused_by_user", "User paused the task",
		"The user paused the task through the task-control API (POST /api/tasks/{id}/control, action=pause). This Planner/Worker run was actively cancelled; the in-flight intent returns to frontier(open) and is claimed again after the task resumes, then executed from the start")
	AbortPausedByOrchestrator = cause("paused_by_orchestrator", "Orchestrator agent paused the task",
		"The orchestrator agent called the pause_task tool to pause this task. This Planner/Worker run was actively cancelled; the in-flight intent returns to frontier(open) and is re-executed after resumption")
	AbortTaskDeleted = cause("task_deleted", "Task deleted",
		"The task is being deleted (DELETE /api/tasks/{id}); the delete barrier has cancelled the Planner, Workers and main Agent running for this task. The results of this run will no longer be used")
	AbortPausedOnReload = cause("paused_on_reload", "Backend restored the task's paused state",
		"On startup the backend restored the task's paused state from the state persisted in the database. This run was cancelled; normally no Agent is running during the restore phase")
	AbortGoalMet = cause("goal_met", "Planner determined the task goal was achieved",
		"The planner determined the task goal was achieved and set the task to done, then cancelled the still-running Workers; those intents are marked stopped rather than failed")
	AbortSettleDrainTimeout = cause("settle_drain_timeout", "Task timeout drain grace exhausted",
		"After the task reached timeout it waited for running Workers to wrap up gracefully, but the 90-second drain grace was still not enough, so a hard cancellation was performed; intents are marked exhausted and facts and assets written during wrap-up are retained")

	// Per-work context.
	AbortKilledByPlanner = cause("killed_by_planner", "Planner terminated this intent",
		"The planner called kill_work to actively terminate this intent, usually meaning the direction drifted or there is no value in continuing; the intent is marked stopped and is not claimed again automatically")
	AbortWorkPausedByUser = cause("work_paused_by_user", "User paused this Worker intent",
		"The user paused the running Worker. This call was cancelled and the intent becomes paused; registered intents, facts, findings and activity records are all retained, and execution restarts from the beginning after resumption")
	AbortWorkCancelledByUser = cause("work_cancelled_by_user", "User deleted this Worker intent",
		"The user deleted the running Worker. This call was cancelled; after the Worker leaves the write section, the server handles the intent according to the deletion mode the user chose — a soft delete only marks it as deleted and keeps all output, while a hard delete cascades away the intent and any downstream nodes supported solely by it")
	AbortWorkFinished = cause("work_finished", "Worker finished normally and released its context",
		"The Worker finished normally and the engine released its context resources in detachWork. This is not an interruption; if it appears in an interruption message, the cancellation and teardown events raced")
	AbortPausedRaceGuard = cause("paused_race_guard", "Refused to start a new run while the task is paused",
		"While the task is paused, the engine refuses to issue a new execution context, preventing a race between claim and pause from starting a Worker anyway; claimed intents return to frontier")

	// Main Agent and standalone conversation contexts.
	AbortChatStoppedByUser = cause("chat_stopped_by_user", "User stopped this conversation turn",
		"The user clicked stop, actively aborting this run of the main Agent or conversation Agent. Activity records already produced are retained, and the next message can still be sent")
	AbortChatPausedWithTask = cause("chat_paused_with_task", "Task pause also aborted the main Agent conversation",
		"When the user paused the task, the running main Agent conversation was cancelled as well. Activity records already produced are retained; this turn's messages are not replayed automatically after the task resumes")
	AbortChatTurnFinished = cause("chat_turn_finished", "Conversation turn finished normally and released its context",
		"This conversation turn finished normally and the server is releasing the turn's context resources. This is not an interruption; if it appears in an interruption message, the cancellation and teardown events raced")

	// Process-level and per-run hard backstop.
	AbortShutdown = cause("shutdown", "Backend process is shutting down",
		"The backend process received SIGINT or SIGTERM and is restarting, updating or shutting down. All running Agents are cancelled; running intents left over after the restart are reset to open and re-executed")
	AbortRunHardTimeout = cause("run_hard_timeout", "Per-run hard-timeout backstop triggered",
		"A single run exceeded the soft wall-clock budget plus the extra grace, meaning a model request or some tool did not return for a long time and wrap-up at a normal turn boundary could not run. Focus on the last tool call before the interruption that never returned")
)

// AbortReason resolves the named cause attached to a cancelled run context.
func AbortReason(ctx context.Context) (code, short, text string, ok bool) {
	c := context.Cause(ctx)
	if c == nil {
		return "", "", "", false
	}
	var ac *AbortCause
	if errors.As(c, &ac) {
		return ac.Code, ac.Short, ac.Text, true
	}
	switch {
	case errors.Is(c, context.DeadlineExceeded):
		return "deadline_exceeded", "Upstream context reached its deadline",
			"Upstream context reached its deadline, but the setter did not attach a named cause via WithTimeoutCause: " + c.Error(), true
	case errors.Is(c, context.Canceled):
		return "canceled_no_cause", "Canceller attached no named cause",
			"Upstream context was cancelled, but the canceller did not attach a named cause via context.WithCancelCause; register a cause in agent/cancelcause.go and wire it into that cancellation site", true
	default:
		return "other", firstLine(c.Error(), 80), c.Error(), true
	}
}
