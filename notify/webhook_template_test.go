package notify

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// This file locks in the **capability boundary** of the generic Webhook template.
//
// This is the only place in the package where "a user-provided string is evaluated as code", so its capabilities must be made explicit:
// what it can and cannot do, and these properties must be pinned with tests - otherwise if someone later casually adds a
// method to the template context or a readFile to the FuncMap, the capability surface silently expands while the diff looks
// like just a harmless little function.

// TestTemplateContextHasNoMethods is the most important one.
//
// text/template calls exported methods ({{.Foo}} can both access a field and call a method). So if the template context
// can reach **any** type with exported methods, those methods are exposed to the template author.
// This feature's context is deliberately all pure data (only exported fields, zero methods).
//
// If this fails: someone added a method to webhookTemplateData / webhookItem.
// Before deciding to allow it, think carefully about whether the template could use that method to read something that should not be exposed.
func TestTemplateContextHasNoMethods(t *testing.T) {
	for _, v := range []any{webhookTemplateData{}, webhookItem{}} {
		typ := reflect.TypeOf(v)
		if n := typ.NumMethod(); n != 0 {
			var names []string
			for i := 0; i < n; i++ {
				names = append(names, typ.Method(i).Name)
			}
			t.Fatalf("%s exposes %d methods (%s): text/template can call them, "+
				"which hands those capabilities to template authors", typ.Name(), n, strings.Join(names, ", "))
		}
	}
}

// TestTemplateFuncsAreMinimal locks in the set of functions exposed to templates.
//
// Every extra function in the FuncMap adds a capability. Currently there are only json / jsons, whose role is to serialize values
// into JSON fragments - no reading files, no making requests, no executing commands.
func TestTemplateFuncsAreMinimal(t *testing.T) {
	var got []string
	for name := range webhookTemplateFuncs {
		got = append(got, name)
	}
	sort.Strings(got)
	want := []string{"json", "jsons"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the template function set changed: got %v, want %v. Before adding a function, confirm it cannot widen the capability surface"+
			" (no file I/O, no network requests, no command execution)", got, want)
	}
}

// TestTemplateCannotReachUnknownData covers out-of-bounds access in templates:
// accessing something nonexistent must fail rather than echo something back; and the failure message must not leak internal data.
func TestTemplateCannotReachUnknownData(t *testing.T) {
	_, err := renderWebhookBody(`{"x": {{.Environment}}, "y": {{.Env}}}`, singleMsg())
	if err == nil {
		t.Fatal("accessing a missing field must return an error")
	}
	// The error must not contain real content from the template context (vulnerability title/summary).
	for _, leak := range []string{"SQL injection", "parameter id"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("the template error leaked message content %q: %v", leak, err)
		}
	}
}

// TestTemplateRenderFailsPermanently: a mistyped template is a configuration error, and retries will not heal it.
// If treated as retryable, one bad template would make every delivery waste three rounds of backoff.
func TestTemplateRenderFailsPermanently(t *testing.T) {
	cfg := map[string]any{
		"url":           "https://example.com/hook",
		"body_template": `{{.Items.`,
	}
	if err := (webhookChannel{}).Validate(cfg); err == nil {
		t.Fatal("a template syntax error must be caught at save time")
	}
	// Even if validation is bypassed and delivery is attempted directly, it must be judged a permanent failure rather than retried repeatedly.
	_, err := (webhookChannel{}).Send(context.Background(), cfg, singleMsg())
	if err == nil || !IsPermanent(err) {
		t.Fatalf("a bad template must be permanent, got %v", err)
	}
}

// TestTemplateCanOnlyProduceJSON covers the constraint that "template render output must be valid JSON".
// It also incidentally blocks usages like "generate plain text with a template to trigger another protocol".
func TestTemplateCanOnlyProduceJSON(t *testing.T) {
	// A valid template passes.
	ok := map[string]any{"url": "https://example.com/hook", "body_template": `{"t":{{json .Title}}}`}
	if err := (webhookChannel{}).Validate(ok); err != nil {
		t.Fatalf("a valid template must pass validation: %v", err)
	}
	// Rendering non-JSON must be rejected (rather than sent as-is).
	bad := map[string]any{"url": "http://127.0.0.1:1/hook", "body_template": `not json {{.Count}}`}
	_, err := (webhookChannel{}).Send(context.Background(), bad, singleMsg())
	if err == nil || !IsPermanent(err) {
		t.Fatalf("a non-JSON render must be permanent, got %v", err)
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("the error must say the problem is JSON, got %v", err)
	}
}
