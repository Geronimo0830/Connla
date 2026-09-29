# Personal Knowledge Base Security Rules

## 1. Trust Boundaries

Uploaded documents, filenames, browser media types, extracted text, document metadata, Markdown, and AI output are untrusted input. Authentication proves identity; every document, card, topic, resource, search result, and review mutation still requires an ownership check.

## 2. Upload Safety

- Allow only configured extensions and detected types.
- Normalize display names; never use a client filename directly as a server path.
- Enforce per-file and per-request size limits before expensive parsing.
- Compute SHA-256 while streaming where practical.
- Store originals outside executable/static asset paths.
- Do not execute macros, embedded scripts, formulas, links, or attachments.
- Reject or quarantine encrypted documents unless an explicit secure workflow exists.

The Document API registers an attachment only after checking that the authenticated user owns it. It accepts TXT, Markdown, PDF, DOC/DOCX, and XLS/XLSX extensions, checks the matching media type and basic binary signatures where available, applies the configured upload-size limit, rereads the stored bytes, and computes SHA-256 on the server. Client-provided hashes are not accepted.

## 3. Parser Isolation

- Apply time, memory, decompressed-size, page, sheet, row, and cell limits.
- Protect against ZIP bombs and recursive embedded files.
- Pin parser versions and monitor their security advisories.
- Treat parser crashes as failed attempts; keep the server and original file intact.
- Sanitize parser error messages before showing them to users.
- Never render extracted HTML without sanitization.

The first deterministic parser accepts only bounded UTF-8 TXT and Markdown input. It rejects NUL-containing text, records stable failure codes and warnings, checks the stored source SHA-256 again before every parse or retry, and strips raw HTML from Markdown-derived plain text. Parser failures preserve both the original attachment and the last successful derived content.

PDF extraction uses a pinned pure-Go parser with the existing input-size and parse-time bounds, plus a 500-page and 16 MiB extracted-text limit. Malformed files fail with a stable safe code; pages without extractable text are recorded as empty or potentially scanned. Browser reading uses the pinned local PDF.js package and same-origin file URLs; no uploaded PDF script is executed as application code.

Modern Office parsing accepts only DOCX and XLSX OOXML packages. It validates archive paths and duplicate entries, bounds compressed input, entry count, per-entry and total expanded size, document blocks, workbook sheets, cells, and derived text, and reads only the required XML parts. Spreadsheet formulas are stored as source text with their cached value and are never evaluated. Macro-enabled formats are not accepted. Legacy DOC and XLS files remain downloadable originals but are explicitly unsupported for parsing, so the application does not invoke LibreOffice, Tika, or another external converter.

## 4. Content Rendering

- Use the existing sanitized Markdown renderer.
- Escape spreadsheet formulas and CSV-style exports when they can be opened by office software.
- Do not fetch external URLs embedded in a document by default.
- PDF previews must use a pinned viewer and a restrictive content security policy.

## 5. AI Privacy

- AI is disabled by default.
- Never send a document to an external provider merely because it was uploaded.
- Before first external transmission, show provider, content scope, purpose, retention implications, and estimated cost, then require explicit opt-in.
- Send the minimum necessary excerpts, not an entire library.
- Keep secrets server-side and redact them from logs and exports.
- Mark AI output as a draft and retain citations to source locators.

## 6. Deletion And Backup

- Destructive UI actions require a specific confirmation describing affected originals, derived content, cards, and history.
- Backups include the database, original resources, configuration needed for restore, and a manifest of hashes.
- Encrypt backups that leave the local machine.
- Perform restore drills against a separate directory or instance.
- Never claim backup success until restored data is verified.

Document deletion is a two-step API operation: the owner first requests an impact plan, then sends a separate deletion request with explicit confirmation. The confirmed operation removes document metadata and derived rows but deliberately retains the immutable source attachment.

## 7. Logging

Allowed by default: IDs, state transitions, duration, byte size, parser version, and stable error codes.

Forbidden by default: document body text, selected quotes, card bodies, passwords, tokens, cookies, provider API keys, and full AI prompts/responses.

Knowledge search runs locally against a replaceable SQLite FTS5 projection. Every result joins back to the authoritative document owner before it is returned; indexed owner fields are defense-in-depth metadata, not authorization. Search queries are bound parameters and highlighted snippets are rendered as text with controlled mark elements, never as trusted HTML.

## 8. Security Acceptance Checks

- Owner A cannot access Owner B data by changing an ID.
- A renamed executable or malformed archive is rejected safely.
- A parser timeout produces a retryable failure without corrupting the original.
- Rendered content cannot execute script or event-handler code.
- Logs and exported diagnostics contain no secrets or document body text.
- AI-off mode makes no outbound AI request.
