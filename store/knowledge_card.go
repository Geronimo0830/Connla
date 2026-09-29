package store

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// KnowledgeCard is an owner-scoped idea with an optional immutable source snapshot.
type KnowledgeCard struct {
	ID                int32
	UID               string
	CreatorID         int32
	Title             string
	Body              string
	CardType          string
	Archived          bool
	SourceDocumentID  *int32
	SourceDocumentUID string
	SourceTitle       string
	SourceQuote       string
	SourceLocator     string
	SourceContentHash string
	CreatedTs         int64
	UpdatedTs         int64
	TopicIDs          []int32
}

// ValidKnowledgeCardType accepts only the five product-defined card kinds.
func ValidKnowledgeCardType(value string) bool {
	switch value {
	case "excerpt", "idea", "question", "summary", "reference":
		return true
	default:
		return false
	}
}

// CreateKnowledgeCard persists a card and its topic/search links together.
func (s *Store) CreateKnowledgeCard(ctx context.Context, card *KnowledgeCard) (*KnowledgeCard, error) {
	if err := s.validateKnowledgeCard(ctx, card); err != nil {
		return nil, err
	}
	tx, err := s.driver.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start card transaction")
	}
	defer func() { _ = tx.Rollback() }()
	query := s.documentQuery(`INSERT INTO knowledge_card
		(uid,creator_id,title,body,card_type,archived,source_document_id,source_document_uid,source_title,source_quote,source_locator,source_content_hash)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`)
	_, err = tx.ExecContext(ctx, query, card.UID, card.CreatorID, strings.TrimSpace(card.Title), strings.TrimSpace(card.Body),
		card.CardType, card.Archived, card.SourceDocumentID, card.SourceDocumentUID, card.SourceTitle,
		card.SourceQuote, card.SourceLocator, card.SourceContentHash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create knowledge card")
	}
	if err := tx.QueryRowContext(ctx, s.documentQuery("SELECT id FROM knowledge_card WHERE uid=? AND creator_id=?"), card.UID, card.CreatorID).Scan(&card.ID); err != nil {
		return nil, errors.Wrap(err, "failed to resolve created card")
	}
	if err := s.replaceCardTopicsTx(ctx, tx, card.ID, card.TopicIDs); err != nil {
		return nil, err
	}
	if s.profile.Driver == "sqlite" {
		if err := upsertCardSearchTx(ctx, tx, card.ID, card.CreatorID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, errors.Wrap(err, "failed to commit card")
	}
	return s.GetKnowledgeCard(ctx, card.UID, card.CreatorID)
}

// GetKnowledgeCard returns a card only within its owner's boundary.
func (s *Store) GetKnowledgeCard(ctx context.Context, uid string, creatorID int32) (*KnowledgeCard, error) {
	card, err := scanKnowledgeCard(s.driver.GetDB().QueryRowContext(ctx,
		s.documentQuery(knowledgeCardSelect+" WHERE c.uid=? AND c.creator_id=?"), uid, creatorID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get knowledge card")
	}
	card.TopicIDs, err = s.ListKnowledgeCardTopicIDs(ctx, card.ID, creatorID)
	return card, err
}

// ListKnowledgeCards lists the owner's cards, newest first.
func (s *Store) ListKnowledgeCards(ctx context.Context, creatorID int32, includeArchived bool) ([]*KnowledgeCard, error) {
	query := knowledgeCardSelect + " WHERE c.creator_id=?"
	if !includeArchived {
		query += " AND c.archived=false"
	}
	query += " ORDER BY c.updated_ts DESC,c.id DESC LIMIT 500"
	rows, err := s.driver.GetDB().QueryContext(ctx, s.documentQuery(query), creatorID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list knowledge cards")
	}
	defer rows.Close()
	var cards []*KnowledgeCard
	for rows.Next() {
		card, err := scanKnowledgeCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, card := range cards {
		card.TopicIDs, err = s.ListKnowledgeCardTopicIDs(ctx, card.ID, creatorID)
		if err != nil {
			return nil, err
		}
	}
	return cards, nil
}

// UpdateKnowledgeCard modifies only editable fields; source evidence cannot change.
func (s *Store) UpdateKnowledgeCard(ctx context.Context, card *KnowledgeCard) (*KnowledgeCard, error) {
	if err := s.validateKnowledgeCard(ctx, card); err != nil {
		return nil, err
	}
	current, err := s.GetKnowledgeCard(ctx, card.UID, card.CreatorID)
	if err != nil || current == nil {
		return current, err
	}
	tx, err := s.driver.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, s.documentQuery(`UPDATE knowledge_card SET title=?,body=?,card_type=?,archived=?,updated_ts=?
		WHERE id=? AND creator_id=?`), strings.TrimSpace(card.Title), strings.TrimSpace(card.Body), card.CardType,
		card.Archived, time.Now().Unix(), current.ID, card.CreatorID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update card")
	}
	if err := requireOneAffected(result, "card is unavailable"); err != nil {
		return nil, err
	}
	if err := s.replaceCardTopicsTx(ctx, tx, current.ID, card.TopicIDs); err != nil {
		return nil, err
	}
	if s.profile.Driver == "sqlite" {
		if err := upsertCardSearchTx(ctx, tx, current.ID, card.CreatorID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, errors.Wrap(err, "failed to commit card update")
	}
	return s.GetKnowledgeCard(ctx, card.UID, card.CreatorID)
}

// ListKnowledgeCardTopicIDs returns the topic IDs assigned to an owned card.
func (s *Store) ListKnowledgeCardTopicIDs(ctx context.Context, cardID, creatorID int32) ([]int32, error) {
	rows, err := s.driver.GetDB().QueryContext(ctx, s.documentQuery(`SELECT ct.topic_id FROM knowledge_card_topic ct
		JOIN knowledge_card c ON c.id=ct.card_id WHERE ct.card_id=? AND c.creator_id=? ORDER BY ct.topic_id`), cardID, creatorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int32
	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CountDocumentKnowledgeCards counts cards whose live source link will be detached.
func (s *Store) CountDocumentKnowledgeCards(ctx context.Context, documentID, creatorID int32) (int32, error) {
	var count int32
	err := s.driver.GetDB().QueryRowContext(ctx, s.documentQuery(
		"SELECT COUNT(*) FROM knowledge_card WHERE source_document_id=? AND creator_id=?"), documentID, creatorID).Scan(&count)
	return count, errors.Wrap(err, "failed to count document knowledge cards")
}

func (s *Store) validateKnowledgeCard(ctx context.Context, card *KnowledgeCard) error {
	if card == nil || card.UID == "" || card.CreatorID <= 0 || strings.TrimSpace(card.Title) == "" ||
		strings.TrimSpace(card.Body) == "" || !ValidKnowledgeCardType(card.CardType) {
		return errors.New("invalid knowledge card")
	}
	if len([]rune(card.Title)) > 200 || len([]rune(card.Body)) > 20000 {
		return errors.New("knowledge card is too long")
	}
	for _, id := range card.TopicIDs {
		topic, err := s.GetKnowledgeTopic(ctx, &FindKnowledgeTopic{ID: &id, CreatorID: card.CreatorID})
		if err != nil {
			return err
		}
		if topic == nil {
			return errors.New("topic does not belong to creator")
		}
	}
	return nil
}

func (s *Store) replaceCardTopicsTx(ctx context.Context, tx *sql.Tx, cardID int32, topicIDs []int32) error {
	if _, err := tx.ExecContext(ctx, s.documentQuery("DELETE FROM knowledge_card_topic WHERE card_id=?"), cardID); err != nil {
		return err
	}
	seen := make(map[int32]bool)
	for _, id := range topicIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if _, err := tx.ExecContext(ctx, s.documentQuery("INSERT INTO knowledge_card_topic(card_id,topic_id) VALUES (?,?)"), cardID, id); err != nil {
			return err
		}
	}
	return nil
}

func upsertCardSearchTx(ctx context.Context, tx *sql.Tx, cardID, creatorID int32) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM knowledge_search WHERE object_type='card' AND object_id=?", strconv.Itoa(int(cardID))); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO knowledge_search(object_type,object_id,creator_id,title,body,topics)
		SELECT 'card',CAST(c.id AS TEXT),CAST(c.creator_id AS TEXT),c.title,c.body,
		COALESCE((SELECT group_concat(t.name,' ') FROM knowledge_card_topic ct JOIN knowledge_topic t ON t.id=ct.topic_id WHERE ct.card_id=c.id),'')
		FROM knowledge_card c WHERE c.id=? AND c.creator_id=? AND c.archived=0`, cardID, creatorID)
	return errors.Wrap(err, "failed to index knowledge card")
}

const knowledgeCardSelect = `SELECT c.id,c.uid,c.creator_id,c.title,c.body,c.card_type,c.archived,c.source_document_id,
	c.source_document_uid,c.source_title,c.source_quote,c.source_locator,c.source_content_hash,c.created_ts,c.updated_ts
	FROM knowledge_card c`

type knowledgeCardScanner interface{ Scan(...any) error }

func scanKnowledgeCard(scanner knowledgeCardScanner) (*KnowledgeCard, error) {
	card := &KnowledgeCard{}
	var sourceID sql.NullInt64
	err := scanner.Scan(&card.ID, &card.UID, &card.CreatorID, &card.Title, &card.Body, &card.CardType,
		&card.Archived, &sourceID, &card.SourceDocumentUID, &card.SourceTitle, &card.SourceQuote,
		&card.SourceLocator, &card.SourceContentHash, &card.CreatedTs, &card.UpdatedTs)
	if sourceID.Valid {
		id := int32(sourceID.Int64)
		card.SourceDocumentID = &id
	}
	return card, err
}
