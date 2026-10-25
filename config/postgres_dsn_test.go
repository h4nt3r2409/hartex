package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPostgresDSNErrorKeepsIdentifiers pins the install-path startup error: when
// neither ARTEX_PG_DSN nor a config file supplies a database, PostgresDSN returns
// an error that propagates verbatim (db.DSN → server.NewManager → the
// `log.Fatalf("open stores: %v", err)` in cmd/artex/main.go). The identifiers an
// operator must act on stay verbatim.
func TestPostgresDSNErrorKeepsIdentifiers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARTEX_PG_DSN", "")
	t.Setenv("ARTEX_CONFIG", filepath.Join(dir, "nope.json"))

	_, _, err := PostgresDSN()
	if err == nil {
		t.Fatal("missing config should error")
	}
	for _, want := range []string{"ARTEX_PG_DSN", "database", "dsn", "host/user/dbname"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("startup error should keep identifier %q verbatim: %q", want, err.Error())
		}
	}
}

// TestPostgresDSNSources pins the three source labels logged at startup.
func TestPostgresDSNSources(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	// Environment-variable source.
	t.Setenv("ARTEX_CONFIG", filepath.Join(dir, "nope.json"))
	t.Setenv("ARTEX_PG_DSN", "postgres://envwins/x")
	if _, source, err := PostgresDSN(); err != nil {
		t.Fatalf("env source: %v", err)
	} else if source != "environment variable ARTEX_PG_DSN" {
		t.Fatalf("env source = %q", source)
	}

	// Config-file dsn source.
	t.Setenv("ARTEX_PG_DSN", "")
	if err := os.WriteFile(cfgPath, []byte(`{"database":{"dsn":"postgres://full/dsn"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARTEX_CONFIG", cfgPath)
	if _, source, err := PostgresDSN(); err != nil {
		t.Fatalf("dsn source: %v", err)
	} else if !strings.Contains(source, "(database.dsn)") {
		t.Fatalf("dsn source = %q", source)
	}

	// Config-file fields source.
	if err := os.WriteFile(cfgPath, []byte(`{"database":{"host":"10.1.2.3","dbname":"d","user":"u"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, source, err := PostgresDSN(); err != nil {
		t.Fatalf("fields source: %v", err)
	} else if !strings.Contains(source, "(database fields)") {
		t.Fatalf("fields source = %q", source)
	}
}
