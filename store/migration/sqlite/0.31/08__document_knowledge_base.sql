-- Personal knowledge base documents keep the attachment as the immutable source.
CREATE TABLE IF NOT EXISTS document (
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

CREATE INDEX IF NOT EXISTS idx_document_creator_status_updated
  ON document(creator_id, status, updated_ts DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_document_creator_sha256 ON document(creator_id, sha256);

-- Extracted content is replaceable and must never own or overwrite the source attachment.
CREATE TABLE IF NOT EXISTS document_content (
  document_id INTEGER PRIMARY KEY,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  plain_text TEXT NOT NULL DEFAULT '',
  structured_json TEXT NOT NULL DEFAULT '{}',
  content_hash TEXT NOT NULL DEFAULT '',
  extractor TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (document_id) REFERENCES document(id) ON DELETE CASCADE
);

-- Parse attempts are append-only operational history.
CREATE TABLE IF NOT EXISTS document_parse_attempt (
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

CREATE INDEX IF NOT EXISTS idx_document_parse_attempt_document_started
  ON document_parse_attempt(document_id, started_ts DESC, id DESC);
