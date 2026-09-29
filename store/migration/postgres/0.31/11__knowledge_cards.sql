CREATE TABLE IF NOT EXISTS knowledge_card (
  id SERIAL PRIMARY KEY,
  uid TEXT NOT NULL UNIQUE,
  creator_id INTEGER NOT NULL,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  card_type TEXT NOT NULL CHECK (card_type IN ('excerpt', 'idea', 'question', 'summary', 'reference')),
  archived BOOLEAN NOT NULL DEFAULT FALSE,
  source_document_id INTEGER DEFAULT NULL,
  source_document_uid TEXT NOT NULL DEFAULT '',
  source_title TEXT NOT NULL DEFAULT '',
  source_quote TEXT NOT NULL DEFAULT '',
  source_locator TEXT NOT NULL DEFAULT '',
  source_content_hash TEXT NOT NULL DEFAULT '',
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  updated_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  FOREIGN KEY (source_document_id) REFERENCES document(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_knowledge_card_creator_updated ON knowledge_card(creator_id, archived, updated_ts DESC, id DESC);
CREATE TABLE IF NOT EXISTS knowledge_card_topic (
  card_id INTEGER NOT NULL,
  topic_id INTEGER NOT NULL,
  PRIMARY KEY (card_id, topic_id),
  FOREIGN KEY (card_id) REFERENCES knowledge_card(id) ON DELETE CASCADE,
  FOREIGN KEY (topic_id) REFERENCES knowledge_topic(id) ON DELETE CASCADE
);
