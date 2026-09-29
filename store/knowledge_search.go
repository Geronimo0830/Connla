package store

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// ErrKnowledgeSearchUnsupported means the configured database has no approved search implementation.
var ErrKnowledgeSearchUnsupported = errors.New("knowledge search requires SQLite FTS5")

// KnowledgeSearchResult is an owner-checked hit from the replaceable search projection.
type KnowledgeSearchResult struct {
	DocumentID       int32
	DocumentUID      string
	CardUID          string
	Title            string
	OriginalFilename string
	Snippet          string
	Rank             float64
}

// KnowledgeSearchFilters constrain owned search hits without changing the FTS projection.
type KnowledgeSearchFilters struct {
	TopicID       *int32
	FileFormat    string
	CreatedFrom   int64
	CreatedBefore int64
}

// SearchKnowledgeCards searches owned, active cards in the same local FTS projection.
func (s *Store) SearchKnowledgeCards(ctx context.Context, creatorID int32, query string, filters KnowledgeSearchFilters, limit int) ([]*KnowledgeSearchResult, error) {
	if s.profile.Driver != "sqlite" {
		return nil, ErrKnowledgeSearchUnsupported
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []*KnowledgeSearchResult{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	match := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
	q := `SELECT c.uid,c.title,snippet(knowledge_search,4,'<mark>','</mark>',' … ',24),bm25(knowledge_search)
		FROM knowledge_search JOIN knowledge_card c ON c.id=CAST(knowledge_search.object_id AS INTEGER)
		WHERE knowledge_search MATCH ? AND knowledge_search.object_type='card'
		AND c.creator_id=? AND CAST(knowledge_search.creator_id AS INTEGER)=? AND c.archived=0`
	args := []any{match, creatorID, creatorID}
	if filters.TopicID != nil {
		q += " AND EXISTS (SELECT 1 FROM knowledge_card_topic ct WHERE ct.card_id=c.id AND ct.topic_id=?)"
		args = append(args, *filters.TopicID)
	}
	if filters.CreatedFrom > 0 {
		q += " AND c.created_ts>=?"
		args = append(args, filters.CreatedFrom)
	}
	if filters.CreatedBefore > 0 {
		q += " AND c.created_ts<?"
		args = append(args, filters.CreatedBefore)
	}
	q += " ORDER BY bm25(knowledge_search),c.updated_ts DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.driver.GetDB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to search knowledge cards")
	}
	defer rows.Close()
	var out []*KnowledgeSearchResult
	for rows.Next() {
		r := &KnowledgeSearchResult{}
		if err := rows.Scan(&r.CardUID, &r.Title, &r.Snippet, &r.Rank); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SearchKnowledgeDocuments searches owned documents, optionally within one owned topic.
func (s *Store) SearchKnowledgeDocuments(ctx context.Context, creatorID int32, query string, filters KnowledgeSearchFilters, limit int) ([]*KnowledgeSearchResult, error) {
	if s.profile.Driver != "sqlite" {
		return nil, ErrKnowledgeSearchUnsupported
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []*KnowledgeSearchResult{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	match := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
	q := `SELECT d.id, d.uid, d.title, d.original_filename,
		snippet(knowledge_search, 4, '<mark>', '</mark>', ' … ', 24), bm25(knowledge_search)
		FROM knowledge_search JOIN document d ON d.id = CAST(knowledge_search.object_id AS INTEGER)
		WHERE knowledge_search MATCH ? AND knowledge_search.object_type = 'document'
		AND d.creator_id = ? AND CAST(knowledge_search.creator_id AS INTEGER) = ?`
	args := []any{match, creatorID, creatorID}
	if filters.TopicID != nil {
		q += " AND EXISTS (SELECT 1 FROM document_topic dt WHERE dt.document_id = d.id AND dt.topic_id = ?)"
		args = append(args, *filters.TopicID)
	}
	if filters.FileFormat != "" {
		q += " AND d.extension=?"
		args = append(args, filters.FileFormat)
	}
	if filters.CreatedFrom > 0 {
		q += " AND d.created_ts>=?"
		args = append(args, filters.CreatedFrom)
	}
	if filters.CreatedBefore > 0 {
		q += " AND d.created_ts<?"
		args = append(args, filters.CreatedBefore)
	}
	q += " ORDER BY bm25(knowledge_search), d.updated_ts DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.driver.GetDB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to search knowledge documents")
	}
	defer rows.Close()
	var out []*KnowledgeSearchResult
	for rows.Next() {
		r := &KnowledgeSearchResult{}
		if err := rows.Scan(&r.DocumentID, &r.DocumentUID, &r.Title, &r.OriginalFilename, &r.Snippet, &r.Rank); err != nil {
			return nil, errors.Wrap(err, "failed to scan knowledge search result")
		}
		out = append(out, r)
	}
	return out, errors.Wrap(rows.Err(), "failed to read knowledge search results")
}

// RebuildKnowledgeSearch deterministically recreates the owner's projection from authoritative rows.
func (s *Store) RebuildKnowledgeSearch(ctx context.Context, creatorID int32) (int32, error) {
	if s.profile.Driver != "sqlite" {
		return 0, ErrKnowledgeSearchUnsupported
	}
	tx, err := s.driver.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, "DELETE FROM knowledge_search WHERE creator_id = ?", strconv.Itoa(int(creatorID))); err != nil {
		return 0, errors.Wrap(err, "failed to clear knowledge search projection")
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO knowledge_search(object_type,object_id,creator_id,title,body,topics)
		SELECT 'document', CAST(d.id AS TEXT), CAST(d.creator_id AS TEXT),
		CASE WHEN d.title = '' THEN d.original_filename ELSE d.title END, c.plain_text,
		COALESCE((SELECT group_concat(t.name, ' ') FROM document_topic dt JOIN knowledge_topic t ON t.id=dt.topic_id WHERE dt.document_id=d.id), '')
		FROM document d JOIN document_content c ON c.document_id=d.id WHERE d.creator_id=?`, creatorID)
	if err != nil {
		return 0, errors.Wrap(err, "failed to rebuild knowledge search projection")
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO knowledge_search(object_type,object_id,creator_id,title,body,topics)
		SELECT 'card',CAST(c.id AS TEXT),CAST(c.creator_id AS TEXT),c.title,c.body,
		COALESCE((SELECT group_concat(t.name,' ') FROM knowledge_card_topic ct JOIN knowledge_topic t ON t.id=ct.topic_id WHERE ct.card_id=c.id),'')
		FROM knowledge_card c WHERE c.creator_id=? AND c.archived=0`, creatorID); err != nil {
		return 0, errors.Wrap(err, "failed to rebuild card search projection")
	}
	if err = tx.Commit(); err != nil {
		return 0, errors.Wrap(err, "failed to commit knowledge search rebuild")
	}
	return int32(n), nil
}

// CountIndexedKnowledgeCards reports the owner's active cards after a rebuild.
func (s *Store) CountIndexedKnowledgeCards(ctx context.Context, creatorID int32) (int32, error) {
	var count int32
	err := s.driver.GetDB().QueryRowContext(ctx, "SELECT COUNT(*) FROM knowledge_card WHERE creator_id=? AND archived=0", creatorID).Scan(&count)
	return count, err
}

func upsertDocumentSearchTx(ctx context.Context, tx *sql.Tx, documentID, creatorID int32) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM knowledge_search WHERE object_type='document' AND object_id=?", strconv.Itoa(int(documentID))); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO knowledge_search(object_type,object_id,creator_id,title,body,topics)
		SELECT 'document',CAST(d.id AS TEXT),CAST(d.creator_id AS TEXT),CASE WHEN d.title='' THEN d.original_filename ELSE d.title END,c.plain_text,
		COALESCE((SELECT group_concat(t.name,' ') FROM document_topic dt JOIN knowledge_topic t ON t.id=dt.topic_id WHERE dt.document_id=d.id),'')
		FROM document d JOIN document_content c ON c.document_id=d.id WHERE d.id=? AND d.creator_id=?`, documentID, creatorID)
	return errors.Wrap(err, "failed to update knowledge search projection")
}
