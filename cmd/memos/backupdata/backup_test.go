package backupdata

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateRestoreAndVerify(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	data := filepath.Join(base, "instance")
	require.NoError(t, os.Mkdir(data, 0700))
	dbPath := filepath.Join(data, "memos_prod.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "PRAGMA journal_mode=WAL")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "CREATE TABLE attachment (storage_type TEXT, reference TEXT)")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "CREATE TABLE document (title TEXT)")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "CREATE VIRTUAL TABLE knowledge_search USING fts5(title, tokenize='trigram')")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "INSERT INTO attachment VALUES ('LOCAL','assets/original.txt')")
	require.NoError(t, err)
	absoluteAsset := filepath.Join(data, "assets", "absolute.txt")
	_, err = db.ExecContext(ctx, "INSERT INTO attachment VALUES ('LOCAL',?)", filepath.ToSlash(absoluteAsset))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "INSERT INTO document VALUES ('知识管理')")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "INSERT INTO knowledge_search VALUES ('知识管理')")
	require.NoError(t, err)
	require.NoError(t, db.Close())
	require.NoError(t, os.MkdirAll(filepath.Join(data, "assets"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(data, "assets", "original.txt"), []byte("immutable original"), 0600))
	require.NoError(t, os.WriteFile(absoluteAsset, []byte("absolute original"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(data, "memos-instance-setting-storage.json"), []byte(`{"storage":"local"}`), 0600))
	backupDir := filepath.Join(base, "backup")
	manifest, err := Create(ctx, data, "memos_prod.db", backupDir)
	require.NoError(t, err)
	require.Len(t, manifest.Files, 4)
	restored := filepath.Join(base, "restored")
	_, err = Restore(ctx, backupDir, restored)
	require.NoError(t, err)
	_, err = Verify(ctx, restored)
	require.NoError(t, err)
	original, err := os.ReadFile(filepath.Join(restored, "assets", "original.txt"))
	require.NoError(t, err)
	require.Equal(t, "immutable original", string(original))
	absoluteOriginal, err := os.ReadFile(filepath.Join(restored, "assets", "absolute.txt"))
	require.NoError(t, err)
	require.Equal(t, "absolute original", string(absoluteOriginal))
	rdb, err := sql.Open("sqlite", filepath.Join(restored, "memos_prod.db"))
	require.NoError(t, err)
	defer rdb.Close()
	var restoredReference string
	require.NoError(t, rdb.QueryRowContext(ctx, "SELECT reference FROM attachment WHERE reference LIKE '%absolute.txt'").Scan(&restoredReference))
	require.Equal(t, filepath.ToSlash(filepath.Join("assets", "absolute.txt")), restoredReference)
	sourceDB, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer sourceDB.Close()
	var sourceReference string
	require.NoError(t, sourceDB.QueryRowContext(ctx, "SELECT reference FROM attachment WHERE reference LIKE '%absolute.txt'").Scan(&sourceReference))
	require.Equal(t, filepath.ToSlash(absoluteAsset), sourceReference)
	var title string
	require.NoError(t, rdb.QueryRowContext(ctx, "SELECT title FROM document").Scan(&title))
	require.Equal(t, "知识管理", title)
	var hit string
	require.NoError(t, rdb.QueryRowContext(ctx, `SELECT title FROM knowledge_search WHERE knowledge_search MATCH '"知识管理"'`).Scan(&hit))
	require.Equal(t, title, hit)
	_, err = Restore(ctx, backupDir, restored)
	require.ErrorContains(t, err, "already exists")
}

func TestRestoreRejectsTamperingAndOutsideFiles(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	data := filepath.Join(base, "source")
	require.NoError(t, os.Mkdir(data, 0700))
	db, err := sql.Open("sqlite", filepath.Join(data, "memos_prod.db"))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "CREATE TABLE attachment (storage_type TEXT, reference TEXT)")
	require.NoError(t, err)
	require.NoError(t, db.Close())
	backupDir := filepath.Join(base, "backup")
	_, err = Create(ctx, data, "memos_prod.db", backupDir)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(backupDir, "memos_prod.db"), []byte("corrupt"), 0600))
	target := filepath.Join(base, "restored")
	_, err = Restore(ctx, backupDir, target)
	require.ErrorContains(t, err, "mismatch")
	_, statErr := os.Stat(target)
	require.True(t, os.IsNotExist(statErr))
	_, err = Create(ctx, data, "memos_prod.db", filepath.Join(data, "nested-backup"))
	require.ErrorContains(t, err, "separate")
}

func TestCreateRejectsUnportableAttachments(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(data, "memos_prod.db"))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "CREATE TABLE attachment (storage_type TEXT, reference TEXT)")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "INSERT INTO attachment VALUES ('S3', '')")
	require.NoError(t, err)
	require.NoError(t, db.Close())
	_, err = Create(ctx, data, "memos_prod.db", filepath.Join(t.TempDir(), "backup"))
	require.ErrorContains(t, err, "S3 attachments")
}

func TestCreateRejectsAbsoluteLocalAttachmentOutsideData(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	data := filepath.Join(base, "data")
	require.NoError(t, os.Mkdir(data, 0700))
	outside := filepath.Join(base, "outside.txt")
	require.NoError(t, os.WriteFile(outside, []byte("outside"), 0600))
	db, err := sql.Open("sqlite", filepath.Join(data, "memos_prod.db"))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "CREATE TABLE attachment (storage_type TEXT, reference TEXT)")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "INSERT INTO attachment VALUES ('LOCAL',?)", filepath.ToSlash(outside))
	require.NoError(t, err)
	require.NoError(t, db.Close())
	_, err = Create(ctx, data, "memos_prod.db", filepath.Join(base, "backup"))
	require.ErrorContains(t, err, "outside data directory")
}
