package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/pkg/errors"
)

// BeginDocumentParse atomically moves an owned document to PARSING and opens a new attempt.
func (s *Store) BeginDocumentParse(ctx context.Context, documentID, creatorID int32, parserVersion string) (int32, error) {
	tx, err := s.driver.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return 0, errors.Wrap(err, "failed to start document parse transaction")
	}
	defer func() { _ = tx.Rollback() }()

	query := "SELECT status FROM document WHERE id = ? AND creator_id = ?"
	if s.profile.Driver != "sqlite" {
		query += " FOR UPDATE"
	}
	var current DocumentStatus
	if err := tx.QueryRowContext(ctx, s.documentQuery(query), documentID, creatorID).Scan(&current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, errors.Wrap(err, "failed to get document for parsing")
	}
	if err := ValidateDocumentStatusTransition(current, DocumentStatusQueued); err != nil {
		return 0, err
	}
	if err := ValidateDocumentStatusTransition(DocumentStatusQueued, DocumentStatusParsing); err != nil {
		return 0, err
	}

	var attempt int32
	if err := tx.QueryRowContext(ctx, s.documentQuery("SELECT COALESCE(MAX(attempt), 0) + 1 FROM document_parse_attempt WHERE document_id = ?"), documentID).Scan(&attempt); err != nil {
		return 0, errors.Wrap(err, "failed to allocate document parse attempt")
	}
	nowSec := time.Now().Unix()
	if _, err := tx.ExecContext(ctx, s.documentQuery(`
		INSERT INTO document_parse_attempt (document_id, attempt, started_ts, result, parser_version)
		VALUES (?, ?, ?, ?, ?)
	`), documentID, attempt, nowSec, DocumentParseResultRunning, parserVersion); err != nil {
		return 0, errors.Wrap(err, "failed to create document parse attempt")
	}
	if _, err := tx.ExecContext(ctx, s.documentQuery(`
		UPDATE document
		SET status = ?, updated_ts = ?, error_code = '', error_message = ''
		WHERE id = ? AND creator_id = ?
	`), DocumentStatusParsing, nowSec, documentID, creatorID); err != nil {
		return 0, errors.Wrap(err, "failed to start document parsing")
	}
	if err := tx.Commit(); err != nil {
		return 0, errors.Wrap(err, "failed to commit document parse start")
	}
	return attempt, nil
}

// CompleteDocumentParse replaces derived content and completes a running attempt atomically.
func (s *Store) CompleteDocumentParse(ctx context.Context, documentID, creatorID, attempt int32, content *DocumentContent, parserVersion string, durationMs int64) error {
	if content == nil || content.DocumentID != documentID || durationMs < 0 {
		return errors.New("invalid completed document parse")
	}
	tx, err := s.driver.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to start document parse completion transaction")
	}
	defer func() { _ = tx.Rollback() }()

	if err := requireParsingDocument(ctx, s, tx, documentID, creatorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, s.documentQuery("DELETE FROM document_content WHERE document_id = ?"), documentID); err != nil {
		return errors.Wrap(err, "failed to replace document content")
	}
	nowSec := time.Now().Unix()
	if _, err := tx.ExecContext(ctx, s.documentQuery(`
		INSERT INTO document_content (
			document_id, created_ts, updated_ts, plain_text, structured_json, content_hash, extractor
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`), documentID, nowSec, nowSec, content.PlainText, content.StructuredJSON, content.ContentHash, content.Extractor); err != nil {
		return errors.Wrap(err, "failed to save document content")
	}
	result, err := tx.ExecContext(ctx, s.documentQuery(`
		UPDATE document_parse_attempt
		SET ended_ts = ?, result = ?, parser_version = ?, duration_ms = ?
		WHERE document_id = ? AND attempt = ? AND result = ?
	`), nowSec, DocumentParseResultSucceeded, parserVersion, durationMs, documentID, attempt, DocumentParseResultRunning)
	if err != nil {
		return errors.Wrap(err, "failed to complete document parse attempt")
	}
	if err := requireOneAffected(result, "document parse attempt is not running"); err != nil {
		return err
	}
	result, err = tx.ExecContext(ctx, s.documentQuery(`
		UPDATE document
		SET status = ?, updated_ts = ?, parser_version = ?, error_code = '', error_message = '', parsed_ts = ?
		WHERE id = ? AND creator_id = ? AND status = ?
	`), DocumentStatusReady, nowSec, parserVersion, nowSec, documentID, creatorID, DocumentStatusParsing)
	if err != nil {
		return errors.Wrap(err, "failed to mark document ready")
	}
	if err := requireOneAffected(result, "document is not parsing"); err != nil {
		return err
	}
	if s.profile.Driver == "sqlite" {
		if err := upsertDocumentSearchTx(ctx, tx, documentID, creatorID); err != nil {
			return err
		}
	}
	return errors.Wrap(tx.Commit(), "failed to commit document parse completion")
}

// FailDocumentParse records a safe failure while preserving any prior derived content.
func (s *Store) FailDocumentParse(ctx context.Context, documentID, creatorID, attempt int32, result DocumentParseResult, parserVersion, errorCode, errorMessage string, durationMs int64) error {
	var status DocumentStatus
	switch result {
	case DocumentParseResultFailed:
		status = DocumentStatusFailed
	case DocumentParseResultUnsupported:
		status = DocumentStatusUnsupported
	default:
		return errors.New("invalid failed document parse result")
	}
	if durationMs < 0 {
		return errors.New("invalid document parse duration")
	}
	tx, err := s.driver.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to start document parse failure transaction")
	}
	defer func() { _ = tx.Rollback() }()
	if err := requireParsingDocument(ctx, s, tx, documentID, creatorID); err != nil {
		return err
	}
	nowSec := time.Now().Unix()
	updated, err := tx.ExecContext(ctx, s.documentQuery(`
		UPDATE document_parse_attempt
		SET ended_ts = ?, result = ?, parser_version = ?, error_code = ?, error_message = ?, duration_ms = ?
		WHERE document_id = ? AND attempt = ? AND result = ?
	`), nowSec, result, parserVersion, errorCode, errorMessage, durationMs, documentID, attempt, DocumentParseResultRunning)
	if err != nil {
		return errors.Wrap(err, "failed to record document parse failure")
	}
	if err := requireOneAffected(updated, "document parse attempt is not running"); err != nil {
		return err
	}
	updated, err = tx.ExecContext(ctx, s.documentQuery(`
		UPDATE document
		SET status = ?, updated_ts = ?, parser_version = ?, error_code = ?, error_message = ?
		WHERE id = ? AND creator_id = ? AND status = ?
	`), status, nowSec, parserVersion, errorCode, errorMessage, documentID, creatorID, DocumentStatusParsing)
	if err != nil {
		return errors.Wrap(err, "failed to mark document parse failure")
	}
	if err := requireOneAffected(updated, "document is not parsing"); err != nil {
		return err
	}
	return errors.Wrap(tx.Commit(), "failed to commit document parse failure")
}

// GetDocumentContent returns replaceable derived content inside an owner boundary.
func (s *Store) GetDocumentContent(ctx context.Context, documentID, creatorID int32) (*DocumentContent, error) {
	content := &DocumentContent{}
	err := s.driver.GetDB().QueryRowContext(ctx, s.documentQuery(`
		SELECT c.document_id, c.created_ts, c.updated_ts, c.plain_text, c.structured_json, c.content_hash, c.extractor
		FROM document_content c
		JOIN document d ON d.id = c.document_id
		WHERE c.document_id = ? AND d.creator_id = ?
	`), documentID, creatorID).Scan(
		&content.DocumentID, &content.CreatedTs, &content.UpdatedTs, &content.PlainText,
		&content.StructuredJSON, &content.ContentHash, &content.Extractor,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get document content")
	}
	return content, nil
}

// ListDocumentParseAttempts returns append-only parse history inside an owner boundary.
func (s *Store) ListDocumentParseAttempts(ctx context.Context, documentID, creatorID int32) ([]*DocumentParseAttempt, error) {
	rows, err := s.driver.GetDB().QueryContext(ctx, s.documentQuery(`
		SELECT a.id, a.document_id, a.attempt, a.started_ts, a.ended_ts, a.result,
			a.parser_version, a.error_code, a.error_message, a.duration_ms
		FROM document_parse_attempt a
		JOIN document d ON d.id = a.document_id
		WHERE a.document_id = ? AND d.creator_id = ?
		ORDER BY a.attempt
	`), documentID, creatorID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list document parse attempts")
	}
	defer rows.Close()
	attempts := []*DocumentParseAttempt{}
	for rows.Next() {
		attempt := &DocumentParseAttempt{}
		if err := rows.Scan(
			&attempt.ID, &attempt.DocumentID, &attempt.Attempt, &attempt.StartedTs, &attempt.EndedTs,
			&attempt.Result, &attempt.ParserVersion, &attempt.ErrorCode, &attempt.ErrorMessage, &attempt.DurationMs,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan document parse attempt")
		}
		attempts = append(attempts, attempt)
	}
	return attempts, errors.Wrap(rows.Err(), "failed to list document parse attempt rows")
}

func requireParsingDocument(ctx context.Context, s *Store, tx *sql.Tx, documentID, creatorID int32) error {
	query := "SELECT status FROM document WHERE id = ? AND creator_id = ?"
	if s.profile.Driver != "sqlite" {
		query += " FOR UPDATE"
	}
	var status DocumentStatus
	if err := tx.QueryRowContext(ctx, s.documentQuery(query), documentID, creatorID).Scan(&status); err != nil {
		return errors.Wrap(err, "failed to get parsing document")
	}
	if status != DocumentStatusParsing {
		return errors.New("document is not parsing")
	}
	return nil
}

func requireOneAffected(result sql.Result, message string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to inspect document parse update")
	}
	if count != 1 {
		return errors.New(message)
	}
	return nil
}
