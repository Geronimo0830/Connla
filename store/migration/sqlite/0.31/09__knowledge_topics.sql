CREATE TABLE IF NOT EXISTS knowledge_topic (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  creator_id INTEGER NOT NULL,
  parent_id INTEGER DEFAULT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  FOREIGN KEY (parent_id) REFERENCES knowledge_topic(id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_topic_sibling_name
  ON knowledge_topic(creator_id, COALESCE(parent_id, 0), name);
CREATE INDEX IF NOT EXISTS idx_knowledge_topic_creator_parent ON knowledge_topic(creator_id, parent_id, name);

CREATE TABLE IF NOT EXISTS document_topic (
  document_id INTEGER NOT NULL,
  topic_id INTEGER NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  PRIMARY KEY (document_id, topic_id),
  FOREIGN KEY (document_id) REFERENCES document(id) ON DELETE CASCADE,
  FOREIGN KEY (topic_id) REFERENCES knowledge_topic(id) ON DELETE CASCADE
);
