# V1 Architecture

## 1. Architecture Decision

Extend the existing Memos monolith instead of creating a separate frontend, API, worker service, or authentication system. This keeps deployment simple and maximizes reuse of tested upstream behavior.

V1 adds deterministic document processing inside the existing application. It does not add Redis, Elasticsearch, a vector database, Python services, or a message broker.

## 2. Component Flow

```text
Browser
  -> existing React application
  -> existing Connect RPC / HTTP server
  -> document application service
       -> existing resource storage (original bytes)
       -> parser provider (derived text)
       -> store layer (metadata, jobs, cards, topics, review)
       -> SQLite FTS5 index
```

The parser runs through a narrow interface so a format implementation can change without changing the API or data model.

## 3. Repository Placement

Planned placement follows upstream layering:

- `proto/api/v1/`: compatible API contracts; generated files are never edited manually
- `server/router/api/v1/`: transport validation and access control
- `core/`: document, card, topic, and review business rules
- `store/`: queries and transactions
- `store/migration/`: equivalent SQLite, MySQL, and PostgreSQL migrations
- `provider/parser/`: safe format detection and text extraction adapters
- `web/src/pages/`: inbox, reader, cards, topics, search, and review routes
- `web/src/components/`: reusable presentation components
- `web/src/hooks/`: React Query server-state hooks

Exact paths must be confirmed against the current repository before each implementation task.

## 4. Processing State Machine

```text
uploaded -> queued -> parsing -> ready
                         |       
                         +-> failed -> queued (manual retry)
uploaded -----------------> unsupported
```

Rules:

- Only valid transitions are allowed.
- A job lease or equivalent transaction prevents duplicate active parsing.
- Retry creates or increments an attempt record; it does not replace the original file.
- A crash leaves enough state to retry safely.
- Unsupported means the file was accepted and preserved but no safe parser is available.

## 5. Parser Contract

Each parser receives a read-only resource reference and detected file type. It returns:

- Normalized UTF-8 text
- Structured blocks when available
- Source locators such as PDF page, Word section, or workbook sheet/cell range
- Document metadata
- Non-fatal warnings

The parser must enforce file size, decompression, time, and memory limits. File type is determined from extension plus content sniffing; the browser-provided media type is not trusted.

## 6. Search

SQLite FTS5 is the V1 runtime search engine. The searchable projection contains documents and cards but authoritative data remains in normal tables. Index updates happen in the same transaction when practical; repair and rebuild commands must be idempotent.

Although V1 deployment support targets SQLite, upstream policy still requires equivalent schema migrations for MySQL and PostgreSQL. Features that depend on FTS5 must fail clearly or remain disabled on those drivers until a cross-driver search ADR is approved.

## 7. Deployment

Reuse the existing Memos Docker image shape, non-root runtime, port, configuration system, and persistent data directory. V1 remains a single application container plus persistent storage. Any external parsing binary must be pinned, license-documented, health-checked, and included only after a separate dependency decision.

## 8. Observability

Record structured events for upload, parse start, completion, failure, retry, deletion, index rebuild, and restore. Logs must contain document IDs and error codes but not document body text, credentials, or AI prompts by default.

## 9. Decision Gates

Create an ADR before:

- Adding a heavy parser runtime such as Apache Tika
- Changing authentication or authorization
- Adding a background service or queue
- Adding semantic search or a vector store
- Dropping MySQL/PostgreSQL migration compatibility
- Sending document content to an external AI provider
