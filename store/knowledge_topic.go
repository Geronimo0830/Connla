package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// KnowledgeTopic is an owner-scoped learning subject.
type KnowledgeTopic struct {
	ID          int32
	UID         string
	CreatorID   int32
	ParentID    *int32
	ParentUID   string
	Name        string
	Description string
	CreatedTs   int64
	UpdatedTs   int64
}

// FindKnowledgeTopic selects topics inside one owner boundary.
type FindKnowledgeTopic struct {
	ID        *int32
	UID       *string
	CreatorID int32
}

// CreateKnowledgeTopic creates a topic after validating its parent ownership.
func (s *Store) CreateKnowledgeTopic(ctx context.Context, topic *KnowledgeTopic) (*KnowledgeTopic, error) {
	if topic == nil || topic.UID == "" || topic.CreatorID <= 0 || strings.TrimSpace(topic.Name) == "" {
		return nil, errors.New("invalid knowledge topic")
	}
	if topic.ParentID != nil {
		parent, err := s.GetKnowledgeTopic(ctx, &FindKnowledgeTopic{ID: topic.ParentID, CreatorID: topic.CreatorID})
		if err != nil {
			return nil, err
		}
		if parent == nil {
			return nil, errors.New("topic parent does not belong to creator")
		}
	}
	q := s.documentQuery("INSERT INTO knowledge_topic (uid, creator_id, parent_id, name, description) VALUES (?, ?, ?, ?, ?)")
	if _, err := s.driver.GetDB().ExecContext(ctx, q, topic.UID, topic.CreatorID, topic.ParentID, strings.TrimSpace(topic.Name), strings.TrimSpace(topic.Description)); err != nil {
		return nil, errors.Wrap(err, "failed to create knowledge topic")
	}
	return s.GetKnowledgeTopic(ctx, &FindKnowledgeTopic{UID: &topic.UID, CreatorID: topic.CreatorID})
}

// GetKnowledgeTopic returns a topic only to its owner.
func (s *Store) GetKnowledgeTopic(ctx context.Context, find *FindKnowledgeTopic) (*KnowledgeTopic, error) {
	if find == nil || find.CreatorID <= 0 || (find.ID == nil && find.UID == nil) {
		return nil, errors.New("topic owner and identifier are required")
	}
	q := knowledgeTopicSelect + " WHERE creator_id = ?"
	args := []any{find.CreatorID}
	if find.ID != nil {
		q += " AND id = ?"
		args = append(args, *find.ID)
	} else {
		q += " AND uid = ?"
		args = append(args, *find.UID)
	}
	t, err := scanKnowledgeTopic(s.driver.GetDB().QueryRowContext(ctx, s.documentQuery(q), args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get knowledge topic")
	}
	return t, nil
}

// ListKnowledgeTopics lists all topics owned by one user.
func (s *Store) ListKnowledgeTopics(ctx context.Context, creatorID int32) ([]*KnowledgeTopic, error) {
	rows, err := s.driver.GetDB().QueryContext(ctx, s.documentQuery(knowledgeTopicSelect+" WHERE creator_id = ? ORDER BY name, id"), creatorID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list knowledge topics")
	}
	defer rows.Close()
	var out []*KnowledgeTopic
	for rows.Next() {
		t, err := scanKnowledgeTopic(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, errors.Wrap(rows.Err(), "failed to list knowledge topic rows")
}

// UpdateKnowledgeTopic updates a topic and rejects parent cycles.
func (s *Store) UpdateKnowledgeTopic(ctx context.Context, topic *KnowledgeTopic) (*KnowledgeTopic, error) {
	current, err := s.GetKnowledgeTopic(ctx, &FindKnowledgeTopic{ID: &topic.ID, CreatorID: topic.CreatorID})
	if err != nil || current == nil {
		return current, err
	}
	if topic.ParentID != nil {
		if *topic.ParentID == topic.ID {
			return nil, errors.New("topic parent cycle")
		}
		seen := map[int32]bool{topic.ID: true}
		next := topic.ParentID
		for next != nil {
			if seen[*next] {
				return nil, errors.New("topic parent cycle")
			}
			seen[*next] = true
			p, e := s.GetKnowledgeTopic(ctx, &FindKnowledgeTopic{ID: next, CreatorID: topic.CreatorID})
			if e != nil {
				return nil, e
			}
			if p == nil {
				return nil, errors.New("topic parent does not belong to creator")
			}
			next = p.ParentID
		}
	}
	q := s.documentQuery("UPDATE knowledge_topic SET parent_id = ?, name = ?, description = ?, updated_ts = ? WHERE id = ? AND creator_id = ?")
	if _, err := s.driver.GetDB().ExecContext(ctx, q, topic.ParentID, strings.TrimSpace(topic.Name), strings.TrimSpace(topic.Description), time.Now().Unix(), topic.ID, topic.CreatorID); err != nil {
		return nil, errors.Wrap(err, "failed to update knowledge topic")
	}
	return s.GetKnowledgeTopic(ctx, &FindKnowledgeTopic{ID: &topic.ID, CreatorID: topic.CreatorID})
}

// DeleteKnowledgeTopic deletes only the topic and its document links.
func (s *Store) DeleteKnowledgeTopic(ctx context.Context, id, creatorID int32) error {
	t, err := s.GetKnowledgeTopic(ctx, &FindKnowledgeTopic{ID: &id, CreatorID: creatorID})
	if err != nil || t == nil {
		return err
	}
	var children int
	if err := s.driver.GetDB().QueryRowContext(ctx, s.documentQuery("SELECT COUNT(*) FROM knowledge_topic WHERE parent_id = ? AND creator_id = ?"), id, creatorID).Scan(&children); err != nil {
		return err
	}
	if children > 0 {
		return errors.New("topic has children")
	}
	tx, err := s.driver.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, table := range []string{"document_topic", "knowledge_card_topic"} {
		if _, err = tx.ExecContext(ctx, s.documentQuery("DELETE FROM "+table+" WHERE topic_id = ?"), id); err != nil {
			return errors.Wrap(err, "failed to remove topic links")
		}
	}
	if _, err = tx.ExecContext(ctx, s.documentQuery("DELETE FROM knowledge_topic WHERE id = ? AND creator_id = ?"), id, creatorID); err != nil {
		return errors.Wrap(err, "failed to delete knowledge topic")
	}
	if err = tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit knowledge topic deletion")
	}
	if s.profile.Driver == "sqlite" {
		_, err = s.RebuildKnowledgeSearch(ctx, creatorID)
	}
	return err
}

// SetDocumentTopics replaces an owned document's topic links with owned topics.
func (s *Store) SetDocumentTopics(ctx context.Context, documentID, creatorID int32, topicIDs []int32) error {
	doc, err := s.GetDocument(ctx, &FindDocument{ID: &documentID, CreatorID: creatorID})
	if err != nil || doc == nil {
		return errors.New("document does not belong to creator")
	}
	for _, id := range topicIDs {
		t, e := s.GetKnowledgeTopic(ctx, &FindKnowledgeTopic{ID: &id, CreatorID: creatorID})
		if e != nil {
			return e
		}
		if t == nil {
			return errors.New("topic does not belong to creator")
		}
	}
	tx, err := s.driver.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	p := "?"
	if s.profile.Driver == "postgres" {
		p = "$1"
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM document_topic WHERE document_id = "+p, documentID); err != nil {
		return err
	}
	q := s.documentQuery("INSERT INTO document_topic (document_id, topic_id) VALUES (?, ?)")
	for _, id := range topicIDs {
		if _, err = tx.ExecContext(ctx, q, documentID, id); err != nil {
			return err
		}
	}
	if s.profile.Driver == "sqlite" {
		if err := upsertDocumentSearchTx(ctx, tx, documentID, creatorID); err != nil {
			return err
		}
	}
	return errors.Wrap(tx.Commit(), "failed to commit document topics")
}

// ListDocumentTopicIDs returns topic IDs for an owned document.
func (s *Store) ListDocumentTopicIDs(ctx context.Context, documentID, creatorID int32) ([]int32, error) {
	doc, err := s.GetDocument(ctx, &FindDocument{ID: &documentID, CreatorID: creatorID})
	if err != nil || doc == nil {
		return nil, errors.New("document does not belong to creator")
	}
	rows, err := s.driver.GetDB().QueryContext(ctx, s.documentQuery("SELECT topic_id FROM document_topic WHERE document_id = ? ORDER BY topic_id"), documentID)
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

const knowledgeTopicSelect = "SELECT id, uid, creator_id, parent_id, COALESCE((SELECT uid FROM knowledge_topic parent WHERE parent.id = knowledge_topic.parent_id), ''), name, description, created_ts, updated_ts FROM knowledge_topic"

type knowledgeTopicScanner interface{ Scan(...any) error }

func scanKnowledgeTopic(s knowledgeTopicScanner) (*KnowledgeTopic, error) {
	t := &KnowledgeTopic{}
	err := s.Scan(&t.ID, &t.UID, &t.CreatorID, &t.ParentID, &t.ParentUID, &t.Name, &t.Description, &t.CreatedTs, &t.UpdatedTs)
	return t, err
}
