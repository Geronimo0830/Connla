package backupdata

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	// Register the SQLite driver used by the standalone backup tool.
	_ "modernc.org/sqlite"
)

const format = "memos-sqlite-backup-v1"

// Entry records the size and SHA-256 of one restored file.
type Entry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Manifest describes a complete, independently verifiable instance backup.
type Manifest struct {
	Format    string  `json:"format"`
	CreatedAt string  `json:"created_at"`
	Database  string  `json:"database"`
	Files     []Entry `json:"files"`
}

// Create writes a verified backup at a new destination. Stop the server first so local files and database agree.
func Create(ctx context.Context, dataDir, dbName, destination string) (*Manifest, error) {
	source, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}
	target, err := filepath.Abs(destination)
	if err != nil {
		return nil, err
	}
	if err := validateDBName(dbName); err != nil {
		return nil, err
	}
	if inside(target, source) || inside(source, target) {
		return nil, errors.New("backup and data directories must be separate")
	}
	if _, err := os.Stat(target); err == nil {
		return nil, errors.New("backup destination already exists")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if info, err := os.Lstat(filepath.Join(source, dbName)); err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("source database is missing or not a regular file")
	}
	db, err := sql.Open("sqlite", filepath.Join(source, dbName))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := checkDB(ctx, db); err != nil {
		return nil, err
	}
	localFiles, absoluteRefs, err := referencedFiles(ctx, db, source)
	if err != nil {
		return nil, err
	}
	configs, err := configFiles(source)
	if err != nil {
		return nil, err
	}
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(parent, ".memos-backup-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	manifest := &Manifest{Format: format, CreatedAt: time.Now().UTC().Format(time.RFC3339), Database: dbName}
	// VACUUM INTO produces a transactionally consistent SQLite snapshot, including committed WAL data.
	quoted := strings.ReplaceAll(filepath.Join(stage, dbName), "'", "''")
	if _, err := db.ExecContext(ctx, "VACUUM INTO '"+quoted+"'"); err != nil {
		return nil, errors.Wrap(err, "failed to snapshot SQLite database")
	}
	if err := makeSnapshotPortable(ctx, filepath.Join(stage, dbName), absoluteRefs); err != nil {
		return nil, err
	}
	if err := addEntry(stage, dbName, manifest); err != nil {
		return nil, err
	}
	seenFiles := map[string]bool{dbName: true}
	for _, relative := range append(localFiles, configs...) {
		if seenFiles[relative] {
			continue
		}
		seenFiles[relative] = true
		if err := copyRegular(source, stage, relative); err != nil {
			return nil, err
		}
		if err := addEntry(stage, relative, manifest); err != nil {
			return nil, err
		}
	}
	slices.SortFunc(manifest.Files, func(a, b Entry) int { return strings.Compare(a.Path, b.Path) })
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(stage, "manifest.json"), append(encoded, '\n'), 0600); err != nil {
		return nil, err
	}
	if _, err := Verify(ctx, stage); err != nil {
		return nil, errors.Wrap(err, "backup verification failed")
	}
	if err := os.Rename(stage, target); err != nil {
		return nil, errors.Wrap(err, "failed to publish backup")
	}
	return manifest, nil
}

// Verify checks every hash, the SQLite database, and referenced local originals.
func Verify(ctx context.Context, directory string) (*Manifest, error) {
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, errors.Wrap(err, "invalid backup manifest")
	}
	if m.Format != format {
		return nil, errors.New("unsupported backup format")
	}
	if err := validateDBName(m.Database); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	hasDB := false
	for _, entry := range m.Files {
		rel, err := safeRelative(entry.Path)
		if err != nil {
			return nil, err
		}
		if seen[rel] {
			return nil, errors.New("duplicate backup entry")
		}
		seen[rel] = true
		if rel == m.Database {
			hasDB = true
		}
		info, err := regularFile(root, rel)
		if err != nil {
			return nil, errors.Errorf("backup file missing or not regular: %s", rel)
		}
		if info.Size() != entry.Size {
			return nil, errors.Errorf("backup file size mismatch: %s", rel)
		}
		hash, err := hashFile(filepath.Join(root, rel))
		if err != nil {
			return nil, err
		}
		if hash != entry.SHA256 {
			return nil, errors.Errorf("backup file hash mismatch: %s", rel)
		}
	}
	if !hasDB {
		return nil, errors.New("backup database is not listed")
	}
	db, err := sql.Open("sqlite", filepath.Join(root, m.Database))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := checkDB(ctx, db); err != nil {
		return nil, err
	}
	refs, absoluteRefs, err := referencedFiles(ctx, db, root)
	if err != nil {
		return nil, err
	}
	if len(absoluteRefs) > 0 {
		return nil, errors.New("backup contains nonportable absolute attachment references")
	}
	for _, rel := range refs {
		if !seen[rel] {
			return nil, errors.Errorf("local attachment missing from backup: %s", rel)
		}
	}
	return &m, nil
}

// Restore verifies a backup and copies it into a new target directory, then verifies the restored copy.
func Restore(ctx context.Context, source, target string) (*Manifest, error) {
	from, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	to, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	if inside(to, from) || inside(from, to) {
		return nil, errors.New("restore target and backup must be separate")
	}
	manifest, err := Verify(ctx, from)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(to); err == nil {
		return nil, errors.New("restore target already exists")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	parent := filepath.Dir(to)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(parent, ".memos-restore-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	for _, entry := range manifest.Files {
		if err := copyRegular(from, stage, entry.Path); err != nil {
			return nil, err
		}
	}
	if err := copyRegular(from, stage, "manifest.json"); err != nil {
		return nil, err
	}
	if _, err := Verify(ctx, stage); err != nil {
		return nil, errors.Wrap(err, "restored data failed verification")
	}
	if err := os.Rename(stage, to); err != nil {
		return nil, errors.Wrap(err, "failed to publish restored data")
	}
	return manifest, nil
}

func validateDBName(name string) error {
	if name == "" || name != filepath.Base(name) || !strings.HasSuffix(strings.ToLower(name), ".db") {
		return errors.New("database name must be a .db filename")
	}
	_, err := safeRelative(name)
	return err
}
func inside(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}
func safeRelative(value string) (string, error) {
	if value == "" || filepath.IsAbs(value) || filepath.Clean(value) != value || value == "." || value == ".." || strings.HasPrefix(value, ".."+string(os.PathSeparator)) || strings.Contains(value, ":") {
		return "", errors.New("unsafe backup path")
	}
	return value, nil
}
func referencedFiles(ctx context.Context, db *sql.DB, root string) ([]string, map[string]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT storage_type, reference FROM attachment")
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to inspect attachments")
	}
	defer rows.Close()
	seen := map[string]bool{}
	absoluteRefs := map[string]string{}
	var result []string
	for rows.Next() {
		var kind, ref string
		if err := rows.Scan(&kind, &ref); err != nil {
			return nil, nil, err
		}
		if kind == "S3" {
			return nil, nil, errors.New("S3 attachments are not supported by local backup")
		}
		if ref == "" {
			continue // Database-stored blobs are already in the SQLite snapshot.
		}
		if kind != "LOCAL" {
			return nil, nil, errors.Errorf("external attachment storage %q is not supported by local backup", kind)
		}
		path := filepath.FromSlash(ref)
		var rel string
		if filepath.IsAbs(path) {
			if !inside(path, root) {
				return nil, nil, errors.New("local attachment is outside data directory")
			}
			rel, err = filepath.Rel(root, path)
			if err == nil {
				absoluteRefs[ref] = filepath.ToSlash(rel)
			}
		} else {
			rel = path
		}
		rel, err = safeRelative(rel)
		if err != nil {
			return nil, nil, errors.Wrap(err, "local attachment is outside data directory")
		}
		if seen[rel] {
			continue
		}
		seen[rel] = true
		_, err = regularFile(root, rel)
		if err != nil {
			return nil, nil, errors.Errorf("local attachment missing or not regular: %s", rel)
		}
		result = append(result, rel)
	}
	return result, absoluteRefs, rows.Err()
}

// makeSnapshotPortable rebases old absolute local references in the backup copy only.
// The live database is never modified by backup creation.
func makeSnapshotPortable(ctx context.Context, path string, absoluteRefs map[string]string) error {
	if len(absoluteRefs) == 0 {
		return nil
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return errors.Wrap(err, "failed to open backup snapshot")
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=DELETE"); err != nil {
		return errors.Wrap(err, "failed to set backup journal mode")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to start backup reference rebasing")
	}
	defer tx.Rollback()
	for original, relative := range absoluteRefs {
		if _, err := tx.ExecContext(ctx, "UPDATE attachment SET reference=? WHERE storage_type='LOCAL' AND reference=?", relative, original); err != nil {
			return errors.Wrap(err, "failed to rebase backup attachment reference")
		}
	}
	return errors.Wrap(tx.Commit(), "failed to commit backup reference rebasing")
}
func configFiles(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		n := entry.Name()
		if strings.HasPrefix(n, "memos-") && strings.HasSuffix(n, ".json") {
			info, err := os.Lstat(filepath.Join(root, n))
			if err != nil || !info.Mode().IsRegular() {
				return nil, errors.Errorf("configuration is not regular: %s", n)
			}
			files = append(files, n)
		}
	}
	return files, nil
}
func copyRegular(srcRoot, dstRoot, rel string) error {
	if _, err := safeRelative(rel); err != nil {
		return err
	}
	src := filepath.Join(srcRoot, rel)
	info, err := regularFile(srcRoot, rel)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.Errorf("not a regular file: %s", rel)
	}
	dest := filepath.Join(dstRoot, rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
func regularFile(root, rel string) (os.FileInfo, error) {
	if _, err := safeRelative(rel); err != nil {
		return nil, err
	}
	current := root
	parts := strings.Split(filepath.Clean(rel), string(os.PathSeparator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if i < len(parts)-1 && !info.IsDir() {
			return nil, errors.Errorf("backup path parent is not a directory: %s", rel)
		}
		if i == len(parts)-1 {
			if !info.Mode().IsRegular() {
				return nil, errors.Errorf("backup path is not a regular file: %s", rel)
			}
			return info, nil
		}
	}
	return nil, errors.New("invalid backup path")
}
func addEntry(root, rel string, m *Manifest) error {
	info, err := os.Stat(filepath.Join(root, rel))
	if err != nil {
		return err
	}
	hash, err := hashFile(filepath.Join(root, rel))
	if err != nil {
		return err
	}
	m.Files = append(m.Files, Entry{Path: rel, Size: info.Size(), SHA256: hash})
	return nil
}
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func checkDB(ctx context.Context, db *sql.DB) error {
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		return errors.Wrap(err, "SQLite integrity check failed")
	}
	if result != "ok" {
		return errors.Errorf("SQLite integrity check failed: %s", result)
	}
	return nil
}
