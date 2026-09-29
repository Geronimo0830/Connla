-- Replaceable offline search projection. Authoritative data remains in document tables.
CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_search USING fts5(
  object_type UNINDEXED,
  object_id UNINDEXED,
  creator_id UNINDEXED,
  title,
  body,
  topics,
  tokenize = 'trigram'
);
