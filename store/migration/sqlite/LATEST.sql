-- system_setting
CREATE TABLE system_setting (
  name TEXT NOT NULL,
  value TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  UNIQUE(name)
);

-- user
CREATE TABLE user (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  username TEXT COLLATE BINARY NOT NULL UNIQUE,
  role TEXT NOT NULL DEFAULT 'USER',
  email TEXT COLLATE BINARY DEFAULT NULL,
  nickname TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL,
  avatar_url TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX idx_user_email ON user(email);

-- user_setting
CREATE TABLE user_setting (
  user_id INTEGER NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  UNIQUE(user_id, key)
);

-- space
CREATE TABLE space (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}'
);

-- space membership
CREATE TABLE space_member (
  space_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  status TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('ADMIN', 'USER')),
  PRIMARY KEY (space_id, user_id)
);

CREATE INDEX idx_space_member_user_id ON space_member(user_id, space_id);

-- memo
CREATE TABLE memo (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  creator_id INTEGER NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  content TEXT NOT NULL DEFAULT '',
  visibility TEXT NOT NULL CHECK (visibility IN ('PUBLIC', 'PROTECTED', 'PRIVATE', 'SPACE')) DEFAULT 'PRIVATE',
  pinned INTEGER NOT NULL CHECK (pinned IN (0, 1)) DEFAULT 0,
  payload TEXT NOT NULL DEFAULT '{}',
  space_id INTEGER DEFAULT NULL
);

CREATE INDEX idx_memo_creator_id ON memo(creator_id);
CREATE INDEX idx_memo_space_id ON memo(space_id, row_status, created_ts DESC, id DESC);

-- memo_relation
CREATE TABLE memo_relation (
  memo_id INTEGER NOT NULL,
  related_memo_id INTEGER NOT NULL,
  type TEXT NOT NULL,
  UNIQUE(memo_id, related_memo_id, type)
);

CREATE INDEX idx_memo_relation_related_type_memo
  ON memo_relation(related_memo_id, type, memo_id);

-- attachment
CREATE TABLE attachment (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  creator_id INTEGER NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  filename TEXT NOT NULL DEFAULT '',
  blob BLOB DEFAULT NULL,
  type TEXT NOT NULL DEFAULT '',
  size INTEGER NOT NULL DEFAULT 0,
  memo_id INTEGER,
  storage_type TEXT NOT NULL DEFAULT '',
  reference TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}'
);

-- idp
CREATE TABLE idp (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  identifier_filter TEXT NOT NULL DEFAULT '',
  config TEXT NOT NULL DEFAULT '{}'
);

-- inbox
CREATE TABLE inbox (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  sender_id INTEGER NOT NULL,
  receiver_id INTEGER NOT NULL,
  status TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '{}'
);

-- memo reaction
CREATE TABLE reaction (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  creator_id INTEGER NOT NULL,
  memo_id INTEGER NOT NULL,
  reaction_type TEXT NOT NULL,
  UNIQUE(creator_id, memo_id, reaction_type)
);

-- memo_share
CREATE TABLE memo_share (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  uid        TEXT    NOT NULL UNIQUE,
  memo_id    INTEGER NOT NULL,
  creator_id INTEGER NOT NULL,
  created_ts BIGINT  NOT NULL DEFAULT (strftime('%s', 'now')),
  expires_ts BIGINT  DEFAULT NULL,
  FOREIGN KEY (memo_id) REFERENCES memo(id) ON DELETE CASCADE
);

CREATE INDEX idx_memo_share_memo_id ON memo_share(memo_id);

-- user_identity
CREATE TABLE user_identity (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL,
  provider   TEXT    NOT NULL,
  extern_uid TEXT    NOT NULL,
  created_ts BIGINT  NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT  NOT NULL DEFAULT (strftime('%s', 'now')),
  UNIQUE (provider, extern_uid),
  UNIQUE (user_id, provider)
);

CREATE INDEX idx_user_identity_user_id ON user_identity(user_id);

-- personal knowledge base document
CREATE TABLE document (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  creator_id INTEGER NOT NULL,
  attachment_id INTEGER NOT NULL UNIQUE,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  title TEXT NOT NULL DEFAULT '',
  original_filename TEXT NOT NULL,
  extension TEXT NOT NULL DEFAULT '',
  media_type TEXT NOT NULL DEFAULT '',
  byte_size BIGINT NOT NULL CHECK (byte_size >= 0),
  sha256 TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('UPLOADED', 'QUEUED', 'PARSING', 'READY', 'FAILED', 'UNSUPPORTED')) DEFAULT 'UPLOADED',
  parser_version TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  parsed_ts BIGINT DEFAULT NULL,
  FOREIGN KEY (attachment_id) REFERENCES attachment(id) ON DELETE RESTRICT
);

CREATE INDEX idx_document_creator_status_updated
  ON document(creator_id, status, updated_ts DESC, id DESC);
CREATE INDEX idx_document_creator_sha256 ON document(creator_id, sha256);

CREATE TABLE document_content (
  document_id INTEGER PRIMARY KEY,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  plain_text TEXT NOT NULL DEFAULT '',
  structured_json TEXT NOT NULL DEFAULT '{}',
  content_hash TEXT NOT NULL DEFAULT '',
  extractor TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (document_id) REFERENCES document(id) ON DELETE CASCADE
);

CREATE TABLE document_parse_attempt (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  document_id INTEGER NOT NULL,
  attempt INTEGER NOT NULL CHECK (attempt > 0),
  started_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  ended_ts BIGINT DEFAULT NULL,
  result TEXT NOT NULL CHECK (result IN ('RUNNING', 'SUCCEEDED', 'FAILED', 'UNSUPPORTED')) DEFAULT 'RUNNING',
  parser_version TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  duration_ms BIGINT DEFAULT NULL CHECK (duration_ms IS NULL OR duration_ms >= 0),
  UNIQUE(document_id, attempt),
  FOREIGN KEY (document_id) REFERENCES document(id) ON DELETE CASCADE
);

CREATE INDEX idx_document_parse_attempt_document_started
  ON document_parse_attempt(document_id, started_ts DESC, id DESC);

-- knowledge topics and document classification
CREATE TABLE knowledge_topic (
  id INTEGER PRIMARY KEY AUTOINCREMENT, uid TEXT NOT NULL UNIQUE, creator_id INTEGER NOT NULL,
  parent_id INTEGER DEFAULT NULL, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')), updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  FOREIGN KEY (parent_id) REFERENCES knowledge_topic(id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX idx_knowledge_topic_sibling_name ON knowledge_topic(creator_id, COALESCE(parent_id, 0), name);
CREATE INDEX idx_knowledge_topic_creator_parent ON knowledge_topic(creator_id, parent_id, name);
CREATE TABLE document_topic (
  document_id INTEGER NOT NULL, topic_id INTEGER NOT NULL, created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  PRIMARY KEY (document_id, topic_id), FOREIGN KEY (document_id) REFERENCES document(id) ON DELETE CASCADE,
  FOREIGN KEY (topic_id) REFERENCES knowledge_topic(id) ON DELETE CASCADE
);

CREATE VIRTUAL TABLE knowledge_search USING fts5(
  object_type UNINDEXED, object_id UNINDEXED, creator_id UNINDEXED,
  title, body, topics, tokenize = 'trigram'
);

CREATE TABLE knowledge_card (
  id INTEGER PRIMARY KEY AUTOINCREMENT, uid TEXT NOT NULL UNIQUE, creator_id INTEGER NOT NULL,
  title TEXT NOT NULL, body TEXT NOT NULL,
  card_type TEXT NOT NULL CHECK (card_type IN ('excerpt', 'idea', 'question', 'summary', 'reference')),
  archived INTEGER NOT NULL DEFAULT 0, source_document_id INTEGER DEFAULT NULL,
  source_document_uid TEXT NOT NULL DEFAULT '', source_title TEXT NOT NULL DEFAULT '',
  source_quote TEXT NOT NULL DEFAULT '', source_locator TEXT NOT NULL DEFAULT '', source_content_hash TEXT NOT NULL DEFAULT '',
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')), updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  FOREIGN KEY (source_document_id) REFERENCES document(id) ON DELETE SET NULL
);
CREATE INDEX idx_knowledge_card_creator_updated ON knowledge_card(creator_id, archived, updated_ts DESC, id DESC);
CREATE TABLE knowledge_card_topic (
  card_id INTEGER NOT NULL, topic_id INTEGER NOT NULL, PRIMARY KEY (card_id, topic_id),
  FOREIGN KEY (card_id) REFERENCES knowledge_card(id) ON DELETE CASCADE,
  FOREIGN KEY (topic_id) REFERENCES knowledge_topic(id) ON DELETE CASCADE
);
