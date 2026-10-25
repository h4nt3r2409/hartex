package server

import (
	"os"
	"path/filepath"
	"testing"
)

// Regression test that keeps task_archive_package.go's archive-package deletion preparation
// error messages in English. The check reuses assertNonEmpty; this test fails if a
// message becomes empty.
//
// Call-graph determination (see the errArchiveStagedAndOriginalCoexist constant comment): this
// error is stored in the task_archives.error column by the background archive worker
// runOneTaskArchiveJob (task_archives.go:102) via FailTaskArchiveJob, and TaskArchivesPanel in
// tasks/page.tsx renders {archive.error} directly on screen, so it is user-facing only (zero
// feedback into actool/planner). Sibling errors on the same propagation path (archiveTask,
// validateArchivePath) are already English, so this translation is consistent with them.
//
// stageTaskArchivePackageDelete is a pure file function that never reaches the DB, so on this
// host without a DB it can create the original file and the deletion temp file (.deleting-N)
// simultaneously to drive the actual contradiction branch.
func TestTaskArchiveStagedAndOriginalCoexistErrorLocalized(t *testing.T) {
	dataDir := t.TempDir()
	archivePath := taskArchivePath(dataDir, 21, "42")
	if err := os.MkdirAll(filepath.Dir(archivePath), archiveDirMode); err != nil {
		t.Fatal(err)
	}
	// Create the contradictory state where both the original archive package and the deletion temp file exist.
	if err := os.WriteFile(archivePath, []byte("original"), archiveFileMode); err != nil {
		t.Fatal(err)
	}
	staged := archivePath + ".deleting-21"
	if err := os.WriteFile(staged, []byte("archive"), archiveFileMode); err != nil {
		t.Fatal(err)
	}
	_, moved, err := stageTaskArchivePackageDelete(archivePath, 21)
	if err == nil {
		t.Fatalf("an error is required when the original and deleted temp file coexist (moved=%v)", moved)
	}
	if moved {
		t.Fatal("moved must not be true in the conflicting coexistence state")
	}
	assertNonEmpty(t, "staged_and_original_coexist", err.Error())
}
