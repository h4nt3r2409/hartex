package server

import (
	"time"

	"github.com/Autumn-27/artex/agent"
	"github.com/Autumn-27/artex/db"
)

// Server-side resolution of the retry policy, see the LLM retry design notes. Among the five layers:
//   - Connect / empty response / same-provider safe window "follow the endpoint", and each LLM
//     configuration can override the global default (if a profile field is left empty it inherits
//     the global, and if the global is unset the built-in default is used);
//   - Circuit breaking / intent replay are process-level, with only a single global copy.
//
// The global policy reads one settings row from the DB once; the call sites are all on low-frequency
// paths (building a provider, ending work, saving configuration), not worth adding a cache layer;
// the circuit-breaker parameters are the exception—they are read every time on the failure path, so
// applyRetryPolicy pushes them to the Registry for storage.

// retryPolicy reads the global policy; a nil DB yields the zero policy (all
// layers on their built-in defaults).
func (s *Server) retryPolicy() db.LLMRetryPolicy {
	if s.m == nil || s.m.pg == nil {
		return db.LLMRetryPolicy{}
	}
	return s.m.pg.LLMRetryPolicy()
}

// resolveRetry layers one profile's override on top of the global policy and
// converts the result into the form agent.Config carries. Rules combine field by
// field, so a profile that only pins an interval still inherits the global count.
func resolveRetry(o db.RetryOverride, pol db.LLMRetryPolicy) agent.RetryConfig {
	connect := o.Connect.Or(pol.Connect)
	empty := o.Empty.Or(pol.Empty)
	stream := o.Stream.Or(pol.Stream)
	return agent.RetryConfig{
		// Attempt counts keep the original semantics "0=default / negative=off" here: the SDK's
		// MaxRetries / EmptyResponseRetries are completely isomorphic, so let it parse them itself.
		ConnectAttempts: connect.Attempts, ConnectInterval: connect.Interval(),
		EmptyAttempts: empty.Attempts, EmptyInterval: empty.Interval(),
		StreamAttempts: stream.Attempts, StreamInterval: stream.Interval(),
	}
}

// applyProfileRetry fills cfg.Retry for a profile read from the DB.
func (s *Server) applyProfileRetry(cfg *agent.Config, p *db.LLMProfile) {
	if p == nil {
		return
	}
	cfg.Retry = resolveRetry(p.Retry, s.retryPolicy())
}

// Default values for circuit breaking (round-robin cooldown), consistent with llmpool's
// built-ins — here they are overridden only when "the user configured a value".
// Default values for intent replay are in engine.go's modelErrorRetries / modelErrorRetryBackoff.

// applyRetryPolicy pushes the process-wide layers of the policy into the objects
// that consume them on a hot path: the circuit-breaker registry. Called at
// startup and whenever the policy is saved.
func (s *Server) applyRetryPolicy() {
	pol := s.retryPolicy()
	if s.llmHealth != nil {
		s.llmHealth.SetPolicy(pol.Breaker.Attempts, pol.Breaker.Interval())
	}
}

// modelErrorRetryPolicy resolves the intent-level replay knobs (layer ⑤): how
// many times a model_error work is re-run and how long to back off between runs.
func (e *Engine) modelErrorRetryPolicy() (retries int, backoff time.Duration) {
	retries, backoff = modelErrorRetries, modelErrorRetryBackoff
	if e == nil || e.m == nil || e.m.pg == nil {
		return retries, backoff
	}
	rule := e.m.pg.LLMRetryPolicy().Intent
	if rule.Attempts != 0 {
		retries = max(rule.Attempts, 0)
	}
	if d := rule.Interval(); d > 0 {
		backoff = d
	}
	return retries, backoff
}

// emptyTurnNudgeLimit resolves how many empty-turn continuations one work may
// inject (see steerHooks.Stop). It deliberately reuses layer ②'s knob — "empty response
// retry count": the two are two means to the same end. The SDK layer handles "not a single
// content block", by resending the same request verbatim; here it handles "only thinking,
// neither body text nor tools", by appending an instruction that lets the model continue with
// its existing thinking (resending verbatim is meaningless for this idle spin determined by
// the shape of the context). The empty-detection criteria differ because the SDK judges by
// "whether an event was ever yielded", and thinking increments themselves are events—but when
// the user configures "how many times to retry on an empty response" they mean "if the model
// produced no substantive content, try again", and having both layers share one count matches
// that mental model.
//
// It reads the global policy rather than a profile's override: a run may switch profiles
// mid-way due to failover, and this is the overall volume gate for the whole intent, so it
// should not change with the endpoint. The semantics are isomorphic to the SDK's
// emptyRetries(): 0 = default defaultEmptyTurnNudges; -1 (negative) = disable idle-spin
// continuation; >0 = use that value.
func (e *Engine) emptyTurnNudgeLimit() int {
	if e == nil || e.m == nil || e.m.pg == nil {
		return defaultEmptyTurnNudges
	}
	switch n := e.m.pg.LLMRetryPolicy().Empty.Attempts; {
	case n == 0:
		return defaultEmptyTurnNudges
	case n < 0:
		return 0
	default:
		return n
	}
}
