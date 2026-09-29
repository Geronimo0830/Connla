# Third-party Notices

This document distinguishes software already incorporated into this repository from components that are only being evaluated. The applicable source files and license texts remain authoritative.

## Incorporated

### Memos

- Project: <https://github.com/usememos/memos>
- Baseline: v0.31.0
- License: MIT
- Use: application foundation

The original `LICENSE` file is preserved at the repository root.

### ledongthuc/pdf

- Project: <https://github.com/ledongthuc/pdf>
- Version: `v0.0.0-20260907135840-6c8c28e0e8a0`
- License: BSD-3-Clause
- Use: bounded, page-aware PDF text extraction in the Go service
- Security posture: pure Go, no child process; input size, page count, extracted-text size, and processing time are bounded
- Why needed: the existing service had no PDF parser and must keep extraction deterministic and replaceable

### PDF.js

- Project: <https://github.com/mozilla/pdf.js>
- Version: `6.3.289`
- License: Apache-2.0
- Use: local-worker browser rendering of uploaded PDFs
- Security posture: worker is bundled locally with no CDN dependency; source URLs remain same-origin and PDF JavaScript is not executed
- Why needed: browser-native PDF viewers do not provide a consistent, controllable reader across supported browsers

The repository also contains dependencies declared by upstream package manifests. Their licenses must be reviewed through the normal dependency and release process; this summary does not replace their license texts.

## Evaluated But Not Yet Incorporated

The following candidates are architectural references or possible future dependencies. Listing them here does not mean their code or binary artifacts are included:

| Candidate | Possible use | License to verify before adoption |
| --- | --- | --- |
| Apache Tika | Isolated legacy Office and document extraction | Apache-2.0 |
| Excelize | Go-native Excel extraction | BSD-3-Clause |
| SQLite FTS5 | Local full-text search | SQLite public-domain dedication |

Before incorporation, record the exact version, source URL, license file, distribution obligations, security posture, and reason existing dependencies are insufficient.

## Excluded Code Sources

Projects under strong copyleft licenses such as AGPL may be studied at the product-behavior level, but their source code must not be copied into this private derivative without a separate legal and architectural decision.
