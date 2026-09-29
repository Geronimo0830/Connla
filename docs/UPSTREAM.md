# Upstream Baseline

## Source

- Project: [usememos/memos](https://github.com/usememos/memos)
- License: MIT
- Release: `v0.31.0`
- Release commit: `2b2192d` (`chore(main): release 0.31.0 (#6331)`)
- Connla public repository: a reviewed source snapshot with a new initial commit; it does not contain the private development repository's Git history.

The original application foundation comes from the official Memos release. The MIT license and original copyright notice are preserved in `LICENSE`.

## What We Reuse

- Authentication and user settings
- Markdown memo editing and rendering
- Tags, attachments, resources, and storage providers
- Import/export foundations
- Responsive web layout and Docker packaging
- SQLite, MySQL, and PostgreSQL storage abstractions

## Updating From Upstream

1. Read the target Memos release notes and license changes.
2. Create a local backup and confirm the worktree is clean.
3. Fetch the official Memos source into a separate local branch or checkout. Do not push Connla changes to the Memos repository.
4. Merge or cherry-pick into a dedicated update branch.
5. Resolve conflicts without discarding personal knowledge base modules.
6. Run backend, frontend, migration, and Docker checks.
7. Record the new release, commit, conflicts, and verification result in this file.

Do not perform an upstream update as part of an unrelated feature task.
