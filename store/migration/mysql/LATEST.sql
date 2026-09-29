-- system_setting
CREATE TABLE `system_setting` (
  `name` VARCHAR(256) NOT NULL PRIMARY KEY,
  `value` LONGTEXT NOT NULL,
  `description` TEXT NOT NULL
);

-- user
CREATE TABLE `user` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `row_status` VARCHAR(256) NOT NULL DEFAULT 'NORMAL',
  `username` VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL UNIQUE,
  `role` VARCHAR(256) NOT NULL DEFAULT 'USER',
  `email` VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin DEFAULT NULL,
  `nickname` VARCHAR(256) NOT NULL DEFAULT '',
  `password_hash` VARCHAR(256) NOT NULL,
  `avatar_url` LONGTEXT NOT NULL,
  `description` VARCHAR(256) NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX `idx_user_email` ON `user` (`email`);

-- user_setting
CREATE TABLE `user_setting` (
  `user_id` INT NOT NULL,
  `key` VARCHAR(256) NOT NULL,
  `value` LONGTEXT NOT NULL,
  UNIQUE(`user_id`,`key`)
);

-- space
CREATE TABLE `space` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(256) NOT NULL UNIQUE,
  `title` TEXT NOT NULL,
  `description` TEXT NOT NULL,
  `payload` JSON NOT NULL
);

-- space membership
CREATE TABLE `space_member` (
  `space_id` INT NOT NULL,
  `user_id` INT NOT NULL,
  `status` VARCHAR(256) NOT NULL,
  `role` VARCHAR(256) NOT NULL CHECK (`role` IN ('ADMIN', 'USER')),
  PRIMARY KEY (`space_id`, `user_id`)
);

CREATE INDEX `idx_space_member_user_id` ON `space_member`(`user_id`, `space_id`);

-- memo
CREATE TABLE `memo` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(256) NOT NULL UNIQUE,
  `creator_id` INT NOT NULL,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `row_status` VARCHAR(256) NOT NULL DEFAULT 'NORMAL',
  `content` TEXT NOT NULL,
  `visibility` VARCHAR(256) NOT NULL DEFAULT 'PRIVATE',
  `pinned` BOOLEAN NOT NULL DEFAULT FALSE,
  `payload` JSON NOT NULL,
  `space_id` INT DEFAULT NULL
);

CREATE INDEX `idx_memo_space_id` ON `memo`(`space_id`, `row_status`, `created_ts`, `id`);

-- memo_relation
CREATE TABLE `memo_relation` (
  `memo_id` INT NOT NULL,
  `related_memo_id` INT NOT NULL,
  `type` VARCHAR(256) NOT NULL,
  UNIQUE(`memo_id`,`related_memo_id`,`type`)
);

CREATE INDEX `idx_memo_relation_related_type_memo`
  ON `memo_relation`(`related_memo_id`, `type`, `memo_id`);

-- attachment
CREATE TABLE `attachment` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(256) NOT NULL UNIQUE,
  `creator_id` INT NOT NULL,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `filename` TEXT NOT NULL,
  `blob` MEDIUMBLOB,
  `type` VARCHAR(256) NOT NULL DEFAULT '',
  `size` INT NOT NULL DEFAULT '0',
  `memo_id` INT DEFAULT NULL,
  `storage_type` VARCHAR(256) NOT NULL DEFAULT '',
  `reference` TEXT NOT NULL DEFAULT (''),
  `payload` TEXT NOT NULL
);

-- idp
CREATE TABLE `idp` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(256) NOT NULL UNIQUE,
  `name` TEXT NOT NULL,
  `type` TEXT NOT NULL,
  `identifier_filter` VARCHAR(256) NOT NULL DEFAULT '',
  `config` TEXT NOT NULL
);

-- inbox
CREATE TABLE `inbox` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `sender_id` INT NOT NULL,
  `receiver_id` INT NOT NULL,
  `status` TEXT NOT NULL,
  `message` TEXT NOT NULL
);

-- memo reaction
CREATE TABLE `reaction` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `creator_id` INT NOT NULL,
  `memo_id` INT NOT NULL,
  `reaction_type` VARCHAR(256) NOT NULL,
  UNIQUE(`creator_id`,`memo_id`,`reaction_type`)
);

-- memo_share
CREATE TABLE `memo_share` (
  `id`         INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid`        VARCHAR(255) NOT NULL UNIQUE,
  `memo_id`    INT          NOT NULL,
  `creator_id` INT          NOT NULL,
  `created_ts` BIGINT       NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `expires_ts` BIGINT       DEFAULT NULL,
  FOREIGN KEY (`memo_id`) REFERENCES `memo`(`id`) ON DELETE CASCADE
);

CREATE INDEX `idx_memo_share_memo_id` ON `memo_share`(`memo_id`);

-- user_identity
CREATE TABLE `user_identity` (
  `id`         INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `user_id`    INT          NOT NULL,
  `provider`   VARCHAR(256) NOT NULL,
  `extern_uid` VARCHAR(256) NOT NULL,
  `created_ts` BIGINT       NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `updated_ts` BIGINT       NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  UNIQUE (`provider`, `extern_uid`),
  UNIQUE (`user_id`, `provider`)
);

CREATE INDEX `idx_user_identity_user_id` ON `user_identity`(`user_id`);

-- personal knowledge base document
CREATE TABLE `document` (
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
  FOREIGN KEY (`attachment_id`) REFERENCES `attachment`(`id`) ON DELETE RESTRICT
);

CREATE INDEX `idx_document_creator_status_updated`
  ON `document`(`creator_id`, `status`, `updated_ts` DESC, `id` DESC);
CREATE INDEX `idx_document_creator_sha256` ON `document`(`creator_id`, `sha256`);

CREATE TABLE `document_content` (
  `document_id` INT NOT NULL PRIMARY KEY,
  `created_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `updated_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `plain_text` LONGTEXT NOT NULL DEFAULT (''),
  `structured_json` LONGTEXT NOT NULL DEFAULT ('{}'),
  `content_hash` VARCHAR(64) NOT NULL DEFAULT '',
  `extractor` VARCHAR(256) NOT NULL DEFAULT '',
  FOREIGN KEY (`document_id`) REFERENCES `document`(`id`) ON DELETE CASCADE
);

CREATE TABLE `document_parse_attempt` (
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
  FOREIGN KEY (`document_id`) REFERENCES `document`(`id`) ON DELETE CASCADE
);

CREATE TABLE `knowledge_topic` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY, `uid` VARCHAR(256) NOT NULL UNIQUE, `creator_id` INT NOT NULL,
  `parent_id` INT DEFAULT NULL, `parent_key` INT AS (IFNULL(`parent_id`, 0)) STORED, `name` VARCHAR(256) NOT NULL,
  `description` TEXT NOT NULL DEFAULT (''), `created_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `updated_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  UNIQUE KEY `idx_knowledge_topic_sibling_name` (`creator_id`, `parent_key`, `name`),
  INDEX `idx_knowledge_topic_creator_parent` (`creator_id`, `parent_id`, `name`),
  FOREIGN KEY (`parent_id`) REFERENCES `knowledge_topic`(`id`) ON DELETE RESTRICT
);
CREATE TABLE `document_topic` (
  `document_id` INT NOT NULL, `topic_id` INT NOT NULL, `created_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  PRIMARY KEY (`document_id`, `topic_id`), FOREIGN KEY (`document_id`) REFERENCES `document`(`id`) ON DELETE CASCADE,
  FOREIGN KEY (`topic_id`) REFERENCES `knowledge_topic`(`id`) ON DELETE CASCADE
);

CREATE INDEX `idx_document_parse_attempt_document_started`
  ON `document_parse_attempt`(`document_id`, `started_ts` DESC, `id` DESC);

CREATE TABLE `knowledge_card` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY, `uid` VARCHAR(256) NOT NULL UNIQUE, `creator_id` INT NOT NULL,
  `title` TEXT NOT NULL, `body` LONGTEXT NOT NULL,
  `card_type` VARCHAR(32) NOT NULL CHECK (`card_type` IN ('excerpt', 'idea', 'question', 'summary', 'reference')),
  `archived` BOOLEAN NOT NULL DEFAULT FALSE, `source_document_id` INT DEFAULT NULL,
  `source_document_uid` VARCHAR(256) NOT NULL DEFAULT '', `source_title` TEXT NOT NULL,
  `source_quote` LONGTEXT NOT NULL, `source_locator` VARCHAR(256) NOT NULL DEFAULT '',
  `source_content_hash` VARCHAR(64) NOT NULL DEFAULT '',
  `created_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()), `updated_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  INDEX `idx_knowledge_card_creator_updated` (`creator_id`, `archived`, `updated_ts` DESC, `id` DESC),
  FOREIGN KEY (`source_document_id`) REFERENCES `document`(`id`) ON DELETE SET NULL
);
CREATE TABLE `knowledge_card_topic` (
  `card_id` INT NOT NULL, `topic_id` INT NOT NULL, PRIMARY KEY (`card_id`, `topic_id`),
  FOREIGN KEY (`card_id`) REFERENCES `knowledge_card`(`id`) ON DELETE CASCADE,
  FOREIGN KEY (`topic_id`) REFERENCES `knowledge_topic`(`id`) ON DELETE CASCADE
);
