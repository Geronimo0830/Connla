CREATE TABLE IF NOT EXISTS `knowledge_card` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(256) NOT NULL UNIQUE,
  `creator_id` INT NOT NULL,
  `title` TEXT NOT NULL,
  `body` LONGTEXT NOT NULL,
  `card_type` VARCHAR(32) NOT NULL CHECK (`card_type` IN ('excerpt', 'idea', 'question', 'summary', 'reference')),
  `archived` BOOLEAN NOT NULL DEFAULT FALSE,
  `source_document_id` INT DEFAULT NULL,
  `source_document_uid` VARCHAR(256) NOT NULL DEFAULT '',
  `source_title` TEXT NOT NULL,
  `source_quote` LONGTEXT NOT NULL,
  `source_locator` VARCHAR(256) NOT NULL DEFAULT '',
  `source_content_hash` VARCHAR(64) NOT NULL DEFAULT '',
  `created_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `updated_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  INDEX `idx_knowledge_card_creator_updated` (`creator_id`, `archived`, `updated_ts` DESC, `id` DESC),
  FOREIGN KEY (`source_document_id`) REFERENCES `document`(`id`) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS `knowledge_card_topic` (
  `card_id` INT NOT NULL,
  `topic_id` INT NOT NULL,
  PRIMARY KEY (`card_id`, `topic_id`),
  FOREIGN KEY (`card_id`) REFERENCES `knowledge_card`(`id`) ON DELETE CASCADE,
  FOREIGN KEY (`topic_id`) REFERENCES `knowledge_topic`(`id`) ON DELETE CASCADE
);
