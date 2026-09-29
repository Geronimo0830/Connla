-- Personal knowledge base documents keep the attachment as the immutable source.
CREATE TABLE IF NOT EXISTS `document` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(256) NOT NULL UNIQUE,
  `creator_id` INT NOT NULL,
  `attachment_id` INT NOT NULL UNIQUE,
  `created_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `updated_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `title` TEXT NOT NULL DEFAULT (''),
  `original_filename` TEXT NOT NULL,
  `extension` VARCHAR(64) NOT NULL DEFAULT '',
  `media_type` VARCHAR(256) NOT NULL DEFAULT '',
  `byte_size` BIGINT NOT NULL CHECK (`byte_size` >= 0),
  `sha256` VARCHAR(64) NOT NULL,
  `status` VARCHAR(32) NOT NULL CHECK (`status` IN ('UPLOADED', 'QUEUED', 'PARSING', 'READY', 'FAILED', 'UNSUPPORTED')) DEFAULT 'UPLOADED',
  `parser_version` VARCHAR(256) NOT NULL DEFAULT '',
  `error_code` VARCHAR(256) NOT NULL DEFAULT '',
  `error_message` TEXT NOT NULL DEFAULT (''),
  `parsed_ts` BIGINT DEFAULT NULL,
  INDEX `idx_document_creator_status_updated` (`creator_id`, `status`, `updated_ts` DESC, `id` DESC),
  INDEX `idx_document_creator_sha256` (`creator_id`, `sha256`),
  FOREIGN KEY (`attachment_id`) REFERENCES `attachment`(`id`) ON DELETE RESTRICT
);

-- Extracted content is replaceable and must never own or overwrite the source attachment.
CREATE TABLE IF NOT EXISTS `document_content` (
  `document_id` INT NOT NULL PRIMARY KEY,
  `created_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `updated_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `plain_text` LONGTEXT NOT NULL DEFAULT (''),
  `structured_json` LONGTEXT NOT NULL DEFAULT ('{}'),
  `content_hash` VARCHAR(64) NOT NULL DEFAULT '',
  `extractor` VARCHAR(256) NOT NULL DEFAULT '',
  FOREIGN KEY (`document_id`) REFERENCES `document`(`id`) ON DELETE CASCADE
);

-- Parse attempts are append-only operational history.
CREATE TABLE IF NOT EXISTS `document_parse_attempt` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `document_id` INT NOT NULL,
  `attempt` INT NOT NULL CHECK (`attempt` > 0),
  `started_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `ended_ts` BIGINT DEFAULT NULL,
  `result` VARCHAR(32) NOT NULL CHECK (`result` IN ('RUNNING', 'SUCCEEDED', 'FAILED', 'UNSUPPORTED')) DEFAULT 'RUNNING',
  `parser_version` VARCHAR(256) NOT NULL DEFAULT '',
  `error_code` VARCHAR(256) NOT NULL DEFAULT '',
  `error_message` TEXT NOT NULL DEFAULT (''),
  `duration_ms` BIGINT DEFAULT NULL CHECK (`duration_ms` IS NULL OR `duration_ms` >= 0),
  UNIQUE (`document_id`, `attempt`),
  INDEX `idx_document_parse_attempt_document_started` (`document_id`, `started_ts` DESC, `id` DESC),
  FOREIGN KEY (`document_id`) REFERENCES `document`(`id`) ON DELETE CASCADE
);
