CREATE TABLE IF NOT EXISTS `knowledge_topic` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(256) NOT NULL UNIQUE,
  `creator_id` INT NOT NULL,
  `parent_id` INT DEFAULT NULL,
  `parent_key` INT AS (IFNULL(`parent_id`, 0)) STORED,
  `name` VARCHAR(256) NOT NULL,
  `description` TEXT NOT NULL DEFAULT (''),
  `created_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `updated_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  UNIQUE KEY `idx_knowledge_topic_sibling_name` (`creator_id`, `parent_key`, `name`),
  INDEX `idx_knowledge_topic_creator_parent` (`creator_id`, `parent_id`, `name`),
  FOREIGN KEY (`parent_id`) REFERENCES `knowledge_topic`(`id`) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS `document_topic` (
  `document_id` INT NOT NULL,
  `topic_id` INT NOT NULL,
  `created_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  PRIMARY KEY (`document_id`, `topic_id`),
  FOREIGN KEY (`document_id`) REFERENCES `document`(`id`) ON DELETE CASCADE,
  FOREIGN KEY (`topic_id`) REFERENCES `knowledge_topic`(`id`) ON DELETE CASCADE
);
