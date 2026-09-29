package store

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// ErrAttachmentRegisteredAsDocument protects the immutable source file of a
// knowledge-base document from attachment cleanup paths.
var ErrAttachmentRegisteredAsDocument = errors.New("attachment is registered as a document source")

// DocumentStatus is the durable processing state of an immutable source document.
type DocumentStatus string

const (
	DocumentStatusUploaded    DocumentStatus = "UPLOADED"
	DocumentStatusQueued      DocumentStatus = "QUEUED"
	DocumentStatusParsing     DocumentStatus = "PARSING"
	DocumentStatusReady       DocumentStatus = "READY"
	DocumentStatusFailed      DocumentStatus = "FAILED"
	DocumentStatusUnsupported DocumentStatus = "UNSUPPORTED"
)

// DocumentParseResult is the append-only outcome recorded for a parse attempt.
type DocumentParseResult string

const (
	DocumentParseResultRunning     DocumentParseResult = "RUNNING"
	DocumentParseResultSucceeded   DocumentParseResult = "SUCCEEDED"
	DocumentParseResultFailed      DocumentParseResult = "FAILED"
	DocumentParseResultUnsupported DocumentParseResult = "UNSUPPORTED"
)

// Document describes an immutable source attachment and its mutable processing metadata.
type Document struct {
	ID            int32
	UID           string
	CreatorID     int32
	AttachmentID  int32
	AttachmentUID string
	CreatedTs     int64
	UpdatedTs     int64

	Title            string
	OriginalFilename string
	Extension        string
	MediaType        string
	ByteSize         int64
	SHA256           string
	Status           DocumentStatus
	ParserVersion    string
	ErrorCode        string
	ErrorMessage     string
	ParsedTs         *int64
}

// FindDocument selects documents within an explicit owner boundary.
type FindDocument struct {
	ID           *int32
	UID          *string
	AttachmentID *int32
	CreatorID    int32
	Limit        *int
	Offset       *int
}

// DocumentContent is replaceable derived data. It never owns the source attachment.
type DocumentContent struct {
	DocumentID     int32
	CreatedTs      int64
	UpdatedTs      int64
	PlainText      string
	StructuredJSON string
	ContentHash    string
	Extractor      string
}

// DocumentParseAttempt is an append-only record of one parser execution.
type DocumentParseAttempt struct {
	ID            int32
	DocumentID    int32
	Attempt       int32
	StartedTs     int64
	EndedTs       *int64
	Result        DocumentParseResult
	ParserVersion string
	ErrorCode     string
	ErrorMessage  string
	DurationMs    *int64
}

// IsValid reports whether the document status can be persisted.
func (status DocumentStatus) IsValid() bool {
	switch status {
	case DocumentStatusUploaded,
		DocumentStatusQueued,
		DocumentStatusParsing,
		DocumentStatusReady,
		DocumentStatusFailed,
		DocumentStatusUnsupported:
		return true
	default:
		return false
	}
}

// IsValid reports whether the parse result can be persisted.
func (result DocumentParseResult) IsValid() bool {
	switch result {
	case DocumentParseResultRunning,
		DocumentParseResultSucceeded,
		DocumentParseResultFailed,
		DocumentParseResultUnsupported:
		return true
	default:
		return false
	}
}

// ValidateDocumentStatusTransition rejects state changes that would skip or rewrite processing history.
func ValidateDocumentStatusTransition(from, to DocumentStatus) error {
	if !from.IsValid() {
		return errors.Errorf("invalid current document status: %q", from)
	}
	if !to.IsValid() {
		return errors.Errorf("invalid target document status: %q", to)
	}
	if from == to {
		return nil
	}

	allowed := map[DocumentStatus]map[DocumentStatus]bool{
		DocumentStatusUploaded: {
			DocumentStatusQueued:      true,
			DocumentStatusUnsupported: true,
		},
		DocumentStatusQueued: {
			DocumentStatusParsing:     true,
			DocumentStatusFailed:      true,
			DocumentStatusUnsupported: true,
		},
		DocumentStatusParsing: {
			DocumentStatusReady:       true,
			DocumentStatusFailed:      true,
			DocumentStatusUnsupported: true,
		},
		DocumentStatusReady: {
			DocumentStatusQueued: true,
		},
		DocumentStatusFailed: {
			DocumentStatusQueued: true,
		},
		DocumentStatusUnsupported: {
			DocumentStatusQueued: true,
		},
	}
	if !allowed[from][to] {
		return errors.Errorf("invalid document status transition: %s -> %s", from, to)
	}
	return nil
}

func (s *Store) attachmentIsDocumentSource(ctx context.Context, attachmentID int32) (bool, error) {
	query := "SELECT EXISTS(SELECT 1 FROM document WHERE attachment_id = ?)"
	if s.profile.Driver == "postgres" {
		query = "SELECT EXISTS(SELECT 1 FROM document WHERE attachment_id = $1)"
	}
	var exists bool
	if err := s.driver.GetDB().QueryRowContext(ctx, query, attachmentID).Scan(&exists); err != nil {
		return false, errors.Wrap(err, "failed to check document source attachment")
	}
	return exists, nil
}

// CreateDocument registers an existing immutable attachment as a document.
func (s *Store) CreateDocument(ctx context.Context, create *Document) (*Document, error) {
	if create == nil || create.UID == "" || create.CreatorID <= 0 || create.AttachmentID <= 0 || !create.Status.IsValid() {
		return nil, errors.New("invalid document")
	}
	var ownsAttachment bool
	ownerQuery := s.documentQuery("SELECT EXISTS(SELECT 1 FROM attachment WHERE id = ? AND creator_id = ?)")
	if err := s.driver.GetDB().QueryRowContext(ctx, ownerQuery, create.AttachmentID, create.CreatorID).Scan(&ownsAttachment); err != nil {
		return nil, errors.Wrap(err, "failed to validate document attachment owner")
	}
	if !ownsAttachment {
		return nil, errors.New("document attachment does not belong to creator")
	}
	query := s.documentQuery(`
		INSERT INTO document (
			uid, creator_id, attachment_id, title, original_filename, extension,
			media_type, byte_size, sha256, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if _, err := s.driver.GetDB().ExecContext(ctx, query,
		create.UID, create.CreatorID, create.AttachmentID, create.Title,
		create.OriginalFilename, create.Extension, create.MediaType,
		create.ByteSize, create.SHA256, create.Status,
	); err != nil {
		return nil, errors.Wrap(err, "failed to create document")
	}
	return s.GetDocument(ctx, &FindDocument{UID: &create.UID, CreatorID: create.CreatorID})
}

// GetDocument returns one document only when it belongs to the requested owner.
func (s *Store) GetDocument(ctx context.Context, find *FindDocument) (*Document, error) {
	if find == nil || find.CreatorID <= 0 || (find.ID == nil && find.UID == nil && find.AttachmentID == nil) {
		return nil, errors.New("document owner and identifier are required")
	}
	query := documentSelect + " WHERE creator_id = ?"
	args := []any{find.CreatorID}
	if find.ID != nil {
		query += " AND id = ?"
		args = append(args, *find.ID)
	} else if find.UID != nil {
		query += " AND uid = ?"
		args = append(args, *find.UID)
	} else {
		query += " AND attachment_id = ?"
		args = append(args, *find.AttachmentID)
	}
	document, err := scanDocument(s.driver.GetDB().QueryRowContext(ctx, s.documentQuery(query), args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get document")
	}
	return document, nil
}

// GetDocumentDeleteImpact counts replaceable rows that a confirmed deletion removes.
func (s *Store) GetDocumentDeleteImpact(ctx context.Context, documentID, creatorID int32) (int32, int32, bool, error) {
	document, err := s.GetDocument(ctx, &FindDocument{ID: &documentID, CreatorID: creatorID})
	if err != nil || document == nil {
		return 0, 0, false, err
	}
	var contentRows, attemptRows int32
	contentQuery := s.documentQuery("SELECT COUNT(*) FROM document_content WHERE document_id = ?")
	if err := s.driver.GetDB().QueryRowContext(ctx, contentQuery, documentID).Scan(&contentRows); err != nil {
		return 0, 0, false, errors.Wrap(err, "failed to count document content")
	}
	attemptQuery := s.documentQuery("SELECT COUNT(*) FROM document_parse_attempt WHERE document_id = ?")
	if err := s.driver.GetDB().QueryRowContext(ctx, attemptQuery, documentID).Scan(&attemptRows); err != nil {
		return 0, 0, false, errors.Wrap(err, "failed to count document parse attempts")
	}
	return contentRows, attemptRows, true, nil
}

// ListDocuments lists documents belonging to exactly one owner.
func (s *Store) ListDocuments(ctx context.Context, find *FindDocument) ([]*Document, error) {
	if find == nil || find.CreatorID <= 0 {
		return nil, errors.New("document owner is required")
	}
	query := documentSelect + " WHERE creator_id = ? ORDER BY updated_ts DESC, id DESC"
	args := []any{find.CreatorID}
	if find.Limit != nil {
		query += " LIMIT ?"
		args = append(args, *find.Limit)
	}
	if find.Offset != nil {
		if find.Limit == nil {
			query += " LIMIT 100"
		}
		query += " OFFSET ?"
		args = append(args, *find.Offset)
	}
	rows, err := s.driver.GetDB().QueryContext(ctx, s.documentQuery(query), args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list documents")
	}
	defer rows.Close()

	documents := []*Document{}
	for rows.Next() {
		document, err := scanDocument(rows)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan document")
		}
		documents = append(documents, document)
	}
	return documents, errors.Wrap(rows.Err(), "failed to list document rows")
}

// UpdateDocumentStatus applies an allowed lifecycle transition within an owner boundary.
func (s *Store) UpdateDocumentStatus(ctx context.Context, documentID, creatorID int32, next DocumentStatus) (*Document, error) {
	document, err := s.GetDocument(ctx, &FindDocument{ID: &documentID, CreatorID: creatorID})
	if err != nil || document == nil {
		return document, err
	}
	if err := ValidateDocumentStatusTransition(document.Status, next); err != nil {
		return nil, err
	}
	query := s.documentQuery("UPDATE document SET status = ?, updated_ts = ? WHERE id = ? AND creator_id = ?")
	if _, err := s.driver.GetDB().ExecContext(ctx, query, next, time.Now().Unix(), documentID, creatorID); err != nil {
		return nil, errors.Wrap(err, "failed to update document status")
	}
	return s.GetDocument(ctx, &FindDocument{ID: &documentID, CreatorID: creatorID})
}

// DeleteOwnedDocument deletes document metadata and derived rows only for its owner.
func (s *Store) DeleteOwnedDocument(ctx context.Context, documentID, creatorID int32) (bool, error) {
	document, err := s.GetDocument(ctx, &FindDocument{ID: &documentID, CreatorID: creatorID})
	if err != nil || document == nil {
		return false, err
	}
	return true, s.DeleteDocument(ctx, document.ID)
}

// DeleteDocument removes replaceable derived data and document metadata while
// deliberately retaining the immutable source attachment.
func (s *Store) DeleteDocument(ctx context.Context, documentID int32) error {
	tx, err := s.driver.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to start document delete transaction")
	}
	defer func() { _ = tx.Rollback() }()

	placeholder := "?"
	if s.profile.Driver == "postgres" {
		placeholder = "$1"
	}
	// Keep the immutable quote and locator snapshot even when the source document is removed.
	// SQLite installations may run with foreign-key enforcement disabled, so do this explicitly.
	if _, err := tx.ExecContext(ctx, "UPDATE knowledge_card SET source_document_id = NULL WHERE source_document_id = "+placeholder, documentID); err != nil {
		return errors.Wrap(err, "failed to detach document from knowledge cards")
	}
	for _, table := range []string{"document_content", "document_parse_attempt", "document"} {
		query := "DELETE FROM " + table + " WHERE "
		if table == "document" {
			query += "id = " + placeholder
		} else {
			query += "document_id = " + placeholder
		}
		if _, err := tx.ExecContext(ctx, query, documentID); err != nil {
			return errors.Wrapf(err, "failed to delete from %s", table)
		}
	}
	if s.profile.Driver == "sqlite" {
		if _, err := tx.ExecContext(ctx, "DELETE FROM knowledge_search WHERE object_type = 'document' AND object_id = ?", strconv.Itoa(int(documentID))); err != nil {
			return errors.Wrap(err, "failed to delete document search projection")
		}
	}
	return errors.Wrap(tx.Commit(), "failed to commit document deletion")
}

const documentSelect = `SELECT
	id, uid, creator_id, attachment_id,
	COALESCE((SELECT uid FROM attachment WHERE attachment.id = document.attachment_id), ''),
	created_ts, updated_ts, title,
	original_filename, extension, media_type, byte_size, sha256, status,
	parser_version, error_code, error_message, parsed_ts
FROM document`

type documentScanner interface {
	Scan(dest ...any) error
}

func scanDocument(scanner documentScanner) (*Document, error) {
	document := &Document{}
	err := scanner.Scan(
		&document.ID, &document.UID, &document.CreatorID, &document.AttachmentID, &document.AttachmentUID,
		&document.CreatedTs, &document.UpdatedTs, &document.Title,
		&document.OriginalFilename, &document.Extension, &document.MediaType,
		&document.ByteSize, &document.SHA256, &document.Status,
		&document.ParserVersion, &document.ErrorCode, &document.ErrorMessage, &document.ParsedTs,
	)
	return document, err
}

func (s *Store) documentQuery(query string) string {
	if s.profile.Driver != "postgres" {
		return query
	}
	for i := 1; strings.Contains(query, "?"); i++ {
		query = strings.Replace(query, "?", "$"+strconv.Itoa(i), 1)
	}
	return query
}
