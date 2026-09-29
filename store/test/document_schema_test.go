package test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lithammer/shortuuid/v4"
	"github.com/stretchr/testify/require"

	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

func TestDocumentSchemaInvariants(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	attachment, err := ts.CreateAttachment(ctx, &store.Attachment{
		UID:       shortuuid.New(),
		CreatorID: 101,
		Filename:  "knowledge.md",
		Blob:      []byte("source of truth"),
		Type:      "text/markdown",
		Size:      15,
		Payload:   &storepb.AttachmentPayload{},
	})
	require.NoError(t, err)

	db := ts.GetDriver().GetDB()
	documentUID := shortuuid.New()
	_, err = db.ExecContext(ctx, documentSQL(`
		INSERT INTO document (
			uid, creator_id, attachment_id, original_filename, extension, media_type, byte_size, sha256
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`), documentUID, attachment.CreatorID, attachment.ID, attachment.Filename, ".md", attachment.Type, attachment.Size, "sha256-fixture")
	require.NoError(t, err)

	var documentID int32
	err = db.QueryRowContext(ctx, documentSQL("SELECT id FROM document WHERE uid = ?"), documentUID).Scan(&documentID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, documentSQL(`
		INSERT INTO document (
			uid, creator_id, attachment_id, original_filename, byte_size, sha256
		) VALUES (?, ?, ?, ?, ?, ?)
	`), shortuuid.New(), attachment.CreatorID, attachment.ID, attachment.Filename, attachment.Size, "duplicate-source")
	require.Error(t, err, "one source attachment must not be registered as multiple documents")

	_, err = db.ExecContext(ctx, documentSQL("UPDATE document SET status = ? WHERE id = ?"), "NOT_A_STATUS", documentID)
	require.Error(t, err, "the database must reject unknown document states")

	_, err = db.ExecContext(ctx, documentSQL(`
		INSERT INTO document_content (document_id, plain_text, structured_json, content_hash, extractor)
		VALUES (?, ?, ?, ?, ?)
	`), documentID, "parsed text", `{}`, "content-hash", "test-parser")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, documentSQL(`
		INSERT INTO document_parse_attempt (document_id, attempt, result, parser_version)
		VALUES (?, ?, ?, ?)
	`), documentID, 1, "SUCCEEDED", "test-parser-v1")
	require.NoError(t, err)

	err = ts.DeleteAttachment(ctx, &store.DeleteAttachment{ID: attachment.ID})
	require.Error(t, err, "a registered source attachment must be protected from deletion")

	_, err = db.ExecContext(ctx, documentSQL("DELETE FROM document_content WHERE document_id = ?"), documentID)
	require.NoError(t, err)
	requireRowCount(ctx, t, ts, "attachment", "id", attachment.ID, 1)

	_, err = db.ExecContext(ctx, documentSQL(`
		INSERT INTO document_content (document_id, plain_text, structured_json, content_hash, extractor)
		VALUES (?, ?, ?, ?, ?)
	`), documentID, "rebuilt text", `{}`, "rebuilt-hash", "test-parser-v2")
	require.NoError(t, err, "derived content must be independently rebuildable")

	require.NoError(t, ts.DeleteDocument(ctx, documentID))
	requireRowCount(ctx, t, ts, "document_content", "document_id", documentID, 0)
	requireRowCount(ctx, t, ts, "document_parse_attempt", "document_id", documentID, 0)
	requireRowCount(ctx, t, ts, "attachment", "id", attachment.ID, 1)
}

func TestDocumentStoreOwnership(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	ownerAttachment, err := ts.CreateAttachment(ctx, &store.Attachment{
		UID: shortuuid.New(), CreatorID: 101, Filename: "owner.txt", Blob: []byte("owner"), Type: "text/plain", Size: 5,
	})
	require.NoError(t, err)
	otherAttachment, err := ts.CreateAttachment(ctx, &store.Attachment{
		UID: shortuuid.New(), CreatorID: 102, Filename: "other.txt", Blob: []byte("other"), Type: "text/plain", Size: 5,
	})
	require.NoError(t, err)
	_, err = ts.CreateDocument(ctx, &store.Document{
		UID: shortuuid.New(), CreatorID: 101, AttachmentID: otherAttachment.ID,
		OriginalFilename: otherAttachment.Filename, Extension: ".txt", MediaType: otherAttachment.Type,
		ByteSize: otherAttachment.Size, SHA256: "wrong-owner-hash", Status: store.DocumentStatusUploaded,
	})
	require.Error(t, err, "store must reject a source attachment owned by another creator")

	ownerDocument, err := ts.CreateDocument(ctx, &store.Document{
		UID: shortuuid.New(), CreatorID: 101, AttachmentID: ownerAttachment.ID,
		OriginalFilename: ownerAttachment.Filename, Extension: ".txt", MediaType: ownerAttachment.Type,
		ByteSize: ownerAttachment.Size, SHA256: "owner-hash", Status: store.DocumentStatusUploaded,
	})
	require.NoError(t, err)
	_, err = ts.CreateDocument(ctx, &store.Document{
		UID: shortuuid.New(), CreatorID: 102, AttachmentID: otherAttachment.ID,
		OriginalFilename: otherAttachment.Filename, Extension: ".txt", MediaType: otherAttachment.Type,
		ByteSize: otherAttachment.Size, SHA256: "other-hash", Status: store.DocumentStatusUploaded,
	})
	require.NoError(t, err)

	ownerDocuments, err := ts.ListDocuments(ctx, &store.FindDocument{CreatorID: 101})
	require.NoError(t, err)
	require.Len(t, ownerDocuments, 1)
	require.Equal(t, ownerAttachment.UID, ownerDocuments[0].AttachmentUID)

	missing, err := ts.GetDocument(ctx, &store.FindDocument{UID: &ownerDocument.UID, CreatorID: 102})
	require.NoError(t, err)
	require.Nil(t, missing)
	updated, err := ts.UpdateDocumentStatus(ctx, ownerDocument.ID, 101, store.DocumentStatusQueued)
	require.NoError(t, err)
	require.Equal(t, store.DocumentStatusQueued, updated.Status)

	found, err := ts.DeleteOwnedDocument(ctx, ownerDocument.ID, 102)
	require.NoError(t, err)
	require.False(t, found)
	found, err = ts.DeleteOwnedDocument(ctx, ownerDocument.ID, 101)
	require.NoError(t, err)
	require.True(t, found)
	storedAttachment, err := ts.GetAttachment(ctx, &store.FindAttachment{ID: &ownerAttachment.ID})
	require.NoError(t, err)
	require.NotNil(t, storedAttachment)
}

func TestDocumentParseLifecycle(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	attachment, err := ts.CreateAttachment(ctx, &store.Attachment{
		UID: shortuuid.New(), CreatorID: 101, Filename: "parse.txt", Blob: []byte("source"), Type: "text/plain", Size: 6,
	})
	require.NoError(t, err)
	document, err := ts.CreateDocument(ctx, &store.Document{
		UID: shortuuid.New(), CreatorID: 101, AttachmentID: attachment.ID,
		OriginalFilename: attachment.Filename, Extension: ".txt", MediaType: attachment.Type,
		ByteSize: attachment.Size, SHA256: "source-hash", Status: store.DocumentStatusUploaded,
	})
	require.NoError(t, err)

	attempt, err := ts.BeginDocumentParse(ctx, document.ID, 101, "text-v1")
	require.NoError(t, err)
	require.Equal(t, int32(1), attempt)
	require.NoError(t, ts.CompleteDocumentParse(ctx, document.ID, 101, attempt, &store.DocumentContent{
		DocumentID: document.ID, PlainText: "parsed", StructuredJSON: `{"warnings":[]}`,
		ContentHash: "content-hash", Extractor: "text-v1",
	}, "text-v1", 3))

	content, err := ts.GetDocumentContent(ctx, document.ID, 101)
	require.NoError(t, err)
	require.Equal(t, "parsed", content.PlainText)
	attempts, err := ts.ListDocumentParseAttempts(ctx, document.ID, 101)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	require.Equal(t, store.DocumentParseResultSucceeded, attempts[0].Result)

	attempt, err = ts.BeginDocumentParse(ctx, document.ID, 101, "text-v1")
	require.NoError(t, err)
	require.Equal(t, int32(2), attempt)
	require.NoError(t, ts.FailDocumentParse(ctx, document.ID, 101, attempt, store.DocumentParseResultFailed, "text-v1", "TIMEOUT", "document parsing timed out", 5))
	content, err = ts.GetDocumentContent(ctx, document.ID, 101)
	require.NoError(t, err)
	require.Equal(t, "parsed", content.PlainText, "a failed retry must preserve the last successful derived content")
	attempts, err = ts.ListDocumentParseAttempts(ctx, document.ID, 101)
	require.NoError(t, err)
	require.Len(t, attempts, 2)
	require.Equal(t, store.DocumentParseResultFailed, attempts[1].Result)
	require.Equal(t, "TIMEOUT", attempts[1].ErrorCode)

	stored, err := ts.GetDocument(ctx, &store.FindDocument{ID: &document.ID, CreatorID: 101})
	require.NoError(t, err)
	require.Equal(t, store.DocumentStatusFailed, stored.Status)
	require.Equal(t, "source-hash", stored.SHA256)
}

func documentSQL(query string) string {
	if getDriverFromEnv() != "postgres" {
		return query
	}
	for i := 1; strings.Contains(query, "?"); i++ {
		query = strings.Replace(query, "?", fmt.Sprintf("$%d", i), 1)
	}
	return query
}

func requireRowCount(ctx context.Context, t *testing.T, ts *store.Store, table, column string, id int32, want int) {
	t.Helper()
	var got int
	query := documentSQL(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", table, column))
	require.NoError(t, ts.GetDriver().GetDB().QueryRowContext(ctx, query, id).Scan(&got))
	require.Equal(t, want, got)
}
