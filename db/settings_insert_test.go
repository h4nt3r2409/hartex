package db

import (
	"fmt"
	"testing"
	"time"
)

// InsertSettingIfAbsent is a safety net for auth.password_hash: the caller's GetSetting check can be
// rendered ineffective by database errors, and concurrent requests can race ahead of it (bcrypt takes tens of
// milliseconds). Therefore the guarantee of "set only once" must rest on the primary-key constraint, not on an if check
// in application code.
func TestInsertSettingIfAbsentDoesNotOverwrite(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	defer d.Close()

	// Use a test-only key so the real auth.password_hash in the development database is not touched.
	key := fmt.Sprintf("test.insert_if_absent.%d", time.Now().UnixNano())
	defer func() { _, _ = d.Exec(`DELETE FROM settings WHERE key=$1`, key) }()

	inserted, err := d.InsertSettingIfAbsent(key, "first")
	if err != nil {
		t.Fatal(err)
	}
	if !inserted {
		t.Fatal("최초 쓰기는 inserted=true 여야 합니다")
	}

	inserted, err = d.InsertSettingIfAbsent(key, "second")
	if err != nil {
		t.Fatal(err)
	}
	if inserted {
		t.Fatal("키가 이미 있으면 inserted=false 여야 합니다")
	}

	got, ok, err := d.GetSetting(key)
	if err != nil || !ok {
		t.Fatalf("GetSetting: ok=%v err=%v", ok, err)
	}
	if got != "first" {
		t.Fatalf("값이 %q 로 덮였습니다. %q 를 유지해야 합니다", got, "first")
	}
}
