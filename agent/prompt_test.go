package agent

import (
	"strings"
	"testing"

	"github.com/Autumn-27/artex/textpolicy"
)

func TestRenderSystemOverrideAndFallback(t *testing.T) {
	t.Cleanup(func() { PromptOverride = nil })

	// no override → built-in default
	PromptOverride = nil
	if got := renderSystem("planner", "DEFAULT", PlannerVars{Goal: "g"}); got != "DEFAULT" {
		t.Fatalf("no override should give default, got %q", got)
	}

	// override → rendered with vars
	PromptOverride = func(k string) (string, bool) {
		if k == "planner" {
			return "Goal: {{.Goal}} Scope: {{.Scope}}", true
		}
		return "", false
	}
	if got := renderSystem("planner", "DEFAULT", PlannerVars{Goal: "Take X", Scope: "*.x.com"}); got != "Goal: Take X Scope: *.x.com" {
		t.Fatalf("override render: %q", got)
	}

	// override referencing a non-catalog var → execution error → fallback to default
	PromptOverride = func(k string) (string, bool) { return "{{.NotInCatalog}}", true }
	if got := renderSystem("planner", "DEFAULT", PlannerVars{Goal: "x"}); got != "DEFAULT" {
		t.Fatalf("bad var should fall back to default, got %q", got)
	}

	// full plannerSystem path: DB body [A] is honored, then the code-owned tail
	// [C] (intermediate-artifact output rules) is ALWAYS appended — editing the body can't drop it.
	PromptOverride = func(k string) (string, bool) { return "PLANNER {{.Goal}}", true }
	got := plannerSystem("Take X", "/data", "/data")
	if !strings.HasPrefix(got, "PLANNER Take X") {
		t.Fatalf("plannerSystem body not honored: %q", got)
	}
	if !strings.Contains(got, "Intermediate artifact output rules") || !strings.Contains(got, "/data") {
		t.Fatalf("plannerSystem missing code-owned artifact tail: %q", got)
	}

	// worker dual-text via {{if .ProxyAddr}} in a user template, plus the code tail:
	// [B] trafficTool present only when RECORDING (caCert set — the MITM is on, so
	// the traffic_* tools exist), [C] artifact spec always present. The trafficTool
	// block is gated on the CA (arg 2), NOT on ProxyAddr — a global egress proxy
	// with capture off routes traffic but records nothing.
	PromptOverride = func(k string) (string, bool) {
		return "{{if .ProxyAddr}}via proxy {{.ProxyAddr}}{{else}}manual{{end}}", true
	}
	recording := workerSystem("127.0.0.1:8080", "/ca.pem", "/data", "/data")
	if !strings.HasPrefix(recording, "via proxy 127.0.0.1:8080") {
		t.Fatalf("worker proxy branch body: %q", recording)
	}
	if !strings.Contains(recording, "traffic_search") {
		t.Fatalf("worker while recording should inject trafficTool: %q", recording)
	}
	if strings.Contains(recording, "traffic_refs") {
		t.Fatalf("worker bypassed shared optional evidence policy: %q", recording)
	}
	if !strings.Contains(recording, "Intermediate artifact output rules") {
		t.Fatalf("worker missing artifact tail: %q", recording)
	}
	// Egress proxy set but capture OFF (no CA): the ProxyAddr template branch still
	// renders, but the trafficTool block must NOT — those tools are not registered.
	egressOnly := workerSystem("127.0.0.1:8080", "", "/data", "/data")
	if !strings.HasPrefix(egressOnly, "via proxy 127.0.0.1:8080") {
		t.Fatalf("worker egress-only branch body: %q", egressOnly)
	}
	if strings.Contains(egressOnly, "traffic_search") {
		t.Fatalf("worker without recording must NOT inject trafficTool: %q", egressOnly)
	}
	noProxy := workerSystem("", "", "/data", "/data")
	if !strings.HasPrefix(noProxy, "manual") {
		t.Fatalf("worker no-proxy branch body: %q", noProxy)
	}
	if strings.Contains(noProxy, "traffic_search") {
		t.Fatalf("worker without proxy must NOT inject trafficTool: %q", noProxy)
	}
}

func TestOutputDirectiveIsEnglishOnly(t *testing.T) {
	got := outputDirective()
	if !strings.Contains(got, "English") {
		t.Fatalf("directive must require English: %q", got)
	}
	for _, forbidden := range []string{"Korean", "Chinese", "Spanish", "Japanese"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("directive must not name %s: %q", forbidden, got)
		}
	}
	if !strings.Contains(got, "another language") {
		t.Fatalf("directive must forbid mirroring the target/material language: %q", got)
	}
}

func TestWrapupAlwaysAppendsDirective(t *testing.T) {
	got := withWrapupLang("operator prompt")
	if !strings.HasSuffix(got, textpolicy.EnglishOutputDirective) {
		t.Fatalf("wrap-up must always append the English directive: %q", got)
	}
}

// Every user-facing role's system prompt must end with the code-owned English
// directive, so a DB-edited body can never drop it from that role.
func TestEveryRoleAppendsDirective(t *testing.T) {
	t.Cleanup(func() { PromptOverride = nil })
	PromptOverride = nil

	cases := []struct {
		name string
		got  string
	}{
		{"chatSystem", chatSystem("reporter", "/data", "/work")},
		{"goalsSystem", goalsSystem("/data", true)},
		{"mainAgentSystem", mainAgentSystem("goal", "/data", "/work")},
		{"plannerSystem", plannerSystem("goal", "/data", "/work")},
		{"workerSystem", workerSystem("127.0.0.1:8080", "/ca.pem", "/data", "/run")},
		{"withWrapupLang", withWrapupLang("operator prompt")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.HasSuffix(tc.got, textpolicy.EnglishOutputDirective) {
				t.Fatalf("%s must append the English directive: %q", tc.name, tc.got)
			}
		})
	}
}
