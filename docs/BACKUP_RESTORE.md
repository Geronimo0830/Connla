# Backup and restore

Windows desktop users can use **Settings → Backup and restore** instead of the commands below. The desktop app briefly stops its owned local server, verifies each backup, and keeps a safety copy of current data before switching to restored data. The commands below remain available for source-based or advanced installations.

This procedure is for a local SQLite instance. Stop Connla before creating the backup. The database snapshot uses SQLite `VACUUM INTO`; stopping the server keeps external attachment files consistent with database references.

From the project directory, with the server stopped:

```sh
go run ./cmd/memos backup create --data /path/to/memos-data --output /path/to/new-backup
go run ./cmd/memos backup restore --input /path/to/new-backup --target /path/to/new-restored-data
```

Use `--db-name` if the database filename is not `memos_prod.db`. Both output paths must not exist, and the backup must be outside the data directory. On Windows, use ordinary absolute Windows paths for these flags. The restored directory can be used as a separate instance data directory after reviewing its configuration.

The backup directory contains the SQLite snapshot, every locally stored attachment referenced by the database, root-level `memos-*.json` deployment configuration, and `manifest.json` with file sizes and SHA-256 hashes. Database-stored attachment blobs are inside the snapshot. The command verifies hashes, SQLite integrity, and referenced files before reporting success. A restore verifies the input and the copied result before publishing the new directory. A failed restore does not overwrite an existing target.

Older local uploads may have an absolute attachment path within the data directory. Backup creation converts those references to portable relative paths in the backup snapshot only; the running database is not rewritten. Absolute paths outside the data directory remain unsupported and cause backup creation to fail safely.

Test a backup by restoring it to a separate location and checking a known original file, one document row, and one search result. Keep backup copies encrypted when they leave the local machine. The manifest detects corruption; it is not a signature proving who created the backup.

External object storage (such as S3) and local attachment references outside the data directory are not portable with this command; backup creation stops with an explicit error. Back those systems up separately before moving an instance. This command does not delete or overwrite the source instance.
