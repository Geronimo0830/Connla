# V1 Data Model

This is the target logical model. Names and types must be reconciled with existing Memos conventions before migrations are written.

## 1. Existing Entities To Reuse

- `user`: owner and access boundary
- `attachment`: original uploaded file and storage metadata (the current Memos name for this concept)
- `memo`: existing Markdown notes; do not silently convert every card into a memo
- tags and workspace settings where their semantics already fit

## 2. New Entities

### `document`

One immutable source attachment plus mutable processing metadata.

| Field | Purpose |
| --- | --- |
| `id` | Stable document identifier |
| `creator_id` | Owner |
| `attachment_id` | Original source attachment |
| `title` | User-editable display title |
| `original_filename` | Name at upload |
| `extension` | Normalized extension |
| `media_type` | Detected media type |
| `byte_size` | Original size |
| `sha256` | Integrity and duplicate hint |
| `status` | Current processing state |
| `parser_version` | Parser identity/version |
| `error_code` | Stable failure category |
| `error_message` | Safe user-facing detail |
| timestamps | Created, updated, parsed |

Constraints: `attachment_id` is unique; owner access is required for every read or mutation.

### `document_content`

Derived, replaceable parsing output.

| Field | Purpose |
| --- | --- |
| `document_id` | Parent document |
| `plain_text` | Normalized searchable text |
| `structured_json` | Versioned blocks and locators |
| `content_hash` | Detect unchanged output |
| `extractor` | Parser implementation |
| timestamps | Creation and update |

Deleting or rebuilding this row never deletes the original attachment.

### `document_parse_attempt`

Append-only operational history: document, attempt number, start/end time, result, parser version, safe error, and duration.

### `knowledge_card`

| Field | Purpose |
| --- | --- |
| `id` | Stable card identifier |
| `uid` | Stable public resource name component |
| `creator_id` | Owner |
| `title` | Card title |
| `body` | Markdown body |
| `card_type` | excerpt, idea, question, summary, reference |
| `source_document_id` | Optional source |
| `source_document_uid`, `source_title` | Durable source snapshot, retained if the document is removed |
| `source_locator` | Exact `text:start:end` Unicode-code-point range in parsed content |
| `source_content_hash` | Parsed-content version used to validate the quote |
| `source_quote` | Optional immutable exact excerpt |
| `archived` | Boolean archive flag; archived cards are excluded from default search |
| timestamps | Created and updated |

### `space`

A broad top-level container owned by a user. If an upstream entity already provides these exact semantics, reuse it instead of creating a duplicate table.

### `topic`

A learning subject with owner, optional space, optional parent topic, name, description, and timestamps. Sibling topic names should be unique within an owner and parent scope.

### Join Tables

- `document_topic`: many-to-many document classification
- `knowledge_card_topic`: many-to-many card classification

The join tables use composite uniqueness and cascade only the relationship when a topic is deleted.

### `card_review_state`

One row per card containing next due time, interval, ease value, repetition count, lapse count, and last review time.

### `card_review_log`

Append-only review event: card, rating, previous and new scheduling values, and timestamp.

## 3. Full-text Projection

The SQLite FTS5 projection includes:

- document title and extracted text
- knowledge card title and body
- denormalized topic and tag names for matching

Search rows store object type and object ID so results can be permission-checked against authoritative tables. Rebuilding the projection must not modify source tables.

## 4. Deletion Policy

- Archive cards by default; hard deletion is an explicit separate action.
- Deleting a topic removes links, not cards or documents.
- Deleting extracted content keeps the original resource and allows re-parsing.
- Deleting a document requires a confirmation that states whether its resource and source-linked cards will be retained, detached, or deleted.
- Review logs are retained unless the associated card is explicitly hard-deleted.

## 5. Migration Rules

- Add equivalent incremental and fresh-install SQL for SQLite, MySQL, and PostgreSQL.
- Use foreign keys, uniqueness, and indexes to enforce invariants where supported.
- Migrations must be forward-only and safe on non-empty databases.
- Backfills must be bounded, restartable, and separately tested.
