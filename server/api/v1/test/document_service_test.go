package test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/lithammer/shortuuid/v4"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	documentcore "github.com/usememos/memos/core/document"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

func TestDocumentServiceOwnershipAndDeleteConfirmation(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()
	ctx := context.Background()
	owner, err := ts.CreateRegularUser(ctx, "document-owner")
	require.NoError(t, err)
	other, err := ts.CreateRegularUser(ctx, "document-other")
	require.NoError(t, err)
	ownerCtx := ts.CreateUserContext(ctx, owner.ID)
	otherCtx := ts.CreateUserContext(ctx, other.ID)

	content := []byte("# Personal knowledge\n")
	attachment, err := ts.Service.CreateAttachment(ownerCtx, &v1pb.CreateAttachmentRequest{Attachment: &v1pb.Attachment{
		Filename: "knowledge.md",
		Type:     "text/markdown",
		Content:  content,
	}})
	require.NoError(t, err)

	document, err := ts.Service.CreateDocument(ownerCtx, &v1pb.CreateDocumentRequest{
		Attachment: attachment.Name,
		Title:      "Knowledge",
	})
	require.NoError(t, err)
	require.Equal(t, attachment.Name, document.SourceAttachment)
	require.Equal(t, v1pb.DocumentStatus_DOCUMENT_STATUS_READY, document.Status)
	wantHash := sha256.Sum256(content)
	require.Equal(t, hex.EncodeToString(wantHash[:]), document.Sha256)
	documentUID := document.Name[len("documents/"):]
	storedDocument, err := ts.Store.GetDocument(ctx, &store.FindDocument{UID: &documentUID, CreatorID: owner.ID})
	require.NoError(t, err)
	derived, err := ts.Store.GetDocumentContent(ctx, storedDocument.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "Personal knowledge", derived.PlainText)
	require.Contains(t, derived.StructuredJSON, "MARKDOWN_FORMATTING_REMOVED")

	_, err = ts.Service.CreateDocument(ownerCtx, &v1pb.CreateDocumentRequest{Attachment: attachment.Name})
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	_, err = ts.Service.CreateDocument(otherCtx, &v1pb.CreateDocumentRequest{Attachment: attachment.Name})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	listed, err := ts.Service.ListDocuments(otherCtx, &v1pb.ListDocumentsRequest{})
	require.NoError(t, err)
	require.Empty(t, listed.Documents)
	_, err = ts.Service.GetDocument(otherCtx, &v1pb.GetDocumentRequest{Name: document.Name})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = ts.Service.RetryDocument(otherCtx, &v1pb.RetryDocumentRequest{Name: document.Name})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = ts.Service.GetDocumentDeletePlan(otherCtx, &v1pb.GetDocumentDeletePlanRequest{Name: document.Name})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = ts.Service.DeleteDocument(otherCtx, &v1pb.DeleteDocumentRequest{Name: document.Name, Confirm: true})
	require.Equal(t, codes.NotFound, status.Code(err))

	retried, err := ts.Service.RetryDocument(ownerCtx, &v1pb.RetryDocumentRequest{Name: document.Name})
	require.NoError(t, err)
	require.Equal(t, v1pb.DocumentStatus_DOCUMENT_STATUS_READY, retried.Status)
	require.Equal(t, document.Sha256, retried.Sha256)
	attempts, err := ts.Store.ListDocumentParseAttempts(ctx, storedDocument.ID, owner.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 2)
	require.Equal(t, store.DocumentParseResultSucceeded, attempts[0].Result)
	require.Equal(t, store.DocumentParseResultSucceeded, attempts[1].Result)
	plan, err := ts.Service.GetDocumentDeletePlan(ownerCtx, &v1pb.GetDocumentDeletePlanRequest{Name: document.Name})
	require.NoError(t, err)
	require.True(t, plan.SourceAttachmentRetained)
	require.Equal(t, attachment.Name, plan.SourceAttachment)
	require.Equal(t, int32(1), plan.DerivedContentRows)
	require.Equal(t, int32(2), plan.ParseAttemptRows)

	_, err = ts.Service.DeleteDocument(ownerCtx, &v1pb.DeleteDocumentRequest{Name: document.Name})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = ts.Service.DeleteDocument(ownerCtx, &v1pb.DeleteDocumentRequest{Name: document.Name, Confirm: true})
	require.NoError(t, err)

	attachmentUID := attachment.Name[len("attachments/"):]
	storedAttachment, err := ts.Store.GetAttachment(ctx, &store.FindAttachment{UID: &attachmentUID})
	require.NoError(t, err)
	require.NotNil(t, storedAttachment, "confirmed document deletion must retain the original attachment")
	_, err = ts.Service.GetDocument(ownerCtx, &v1pb.GetDocumentRequest{Name: document.Name})
	require.Equal(t, codes.NotFound, status.Code(err))
}

type timeoutDocumentParser struct{}

func (timeoutDocumentParser) Parse(context.Context, string, string, []byte) (*documentcore.Result, error) {
	return nil, documentcore.ErrTimeout
}

func TestDocumentParsingFailureAndRetry(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()
	ctx := context.Background()
	user, err := ts.CreateRegularUser(ctx, "document-retry")
	require.NoError(t, err)
	userCtx := ts.CreateUserContext(ctx, user.ID)

	ts.Service.DocumentParser = timeoutDocumentParser{}
	attachment, err := ts.Service.CreateAttachment(userCtx, &v1pb.CreateAttachmentRequest{Attachment: &v1pb.Attachment{
		Filename: "retry.txt", Type: "text/plain", Content: []byte("stable source\r\n"),
	}})
	require.NoError(t, err)
	document, err := ts.Service.CreateDocument(userCtx, &v1pb.CreateDocumentRequest{Attachment: attachment.Name})
	require.NoError(t, err)
	require.Equal(t, v1pb.DocumentStatus_DOCUMENT_STATUS_FAILED, document.Status)
	require.Equal(t, "TIMEOUT", document.ErrorCode)
	originalHash := document.Sha256

	ts.Service.DocumentParser = documentcore.NewParser(ts.Service.MarkdownService, documentcore.DefaultMaxInputBytes)
	retried, err := ts.Service.RetryDocument(userCtx, &v1pb.RetryDocumentRequest{Name: document.Name})
	require.NoError(t, err)
	require.Equal(t, v1pb.DocumentStatus_DOCUMENT_STATUS_READY, retried.Status)
	require.Equal(t, originalHash, retried.Sha256)

	documentUID := document.Name[len("documents/"):]
	storedDocument, err := ts.Store.GetDocument(ctx, &store.FindDocument{UID: &documentUID, CreatorID: user.ID})
	require.NoError(t, err)
	derived, err := ts.Store.GetDocumentContent(ctx, storedDocument.ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, "stable source", derived.PlainText)
	require.Contains(t, derived.StructuredJSON, "NEWLINES_NORMALIZED")
	attempts, err := ts.Store.ListDocumentParseAttempts(ctx, storedDocument.ID, user.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 2)
	require.Equal(t, store.DocumentParseResultFailed, attempts[0].Result)
	require.Equal(t, "TIMEOUT", attempts[0].ErrorCode)
	require.Equal(t, store.DocumentParseResultSucceeded, attempts[1].Result)
}

func TestDocumentParsingRejectsChangedSource(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()
	ctx := context.Background()
	user, err := ts.CreateRegularUser(ctx, "document-integrity")
	require.NoError(t, err)
	userCtx := ts.CreateUserContext(ctx, user.ID)
	_, err = ts.Store.UpsertInstanceSetting(ctx, &storepb.InstanceSetting{
		Key: storepb.InstanceSettingKey_STORAGE,
		Value: &storepb.InstanceSetting_StorageSetting{StorageSetting: &storepb.InstanceStorageSetting{
			StorageType:      storepb.InstanceStorageSetting_LOCAL,
			FilepathTemplate: "assets/{filename}",
		}},
	})
	require.NoError(t, err)

	attachment, err := ts.Service.CreateAttachment(userCtx, &v1pb.CreateAttachmentRequest{Attachment: &v1pb.Attachment{
		Filename: "integrity.txt", Type: "text/plain", Content: []byte("original"),
	}})
	require.NoError(t, err)
	document, err := ts.Service.CreateDocument(userCtx, &v1pb.CreateDocumentRequest{Attachment: attachment.Name})
	require.NoError(t, err)
	require.Equal(t, v1pb.DocumentStatus_DOCUMENT_STATUS_READY, document.Status)
	originalHash := document.Sha256

	attachmentUID := attachment.Name[len("attachments/"):]
	storedAttachment, err := ts.Store.GetAttachment(ctx, &store.FindAttachment{UID: &attachmentUID})
	require.NoError(t, err)
	attachmentPath := filepath.FromSlash(storedAttachment.Reference)
	if !filepath.IsAbs(attachmentPath) {
		attachmentPath = filepath.Join(ts.Profile.Data, attachmentPath)
	}
	err = os.WriteFile(attachmentPath, []byte("tampered"), 0o644)
	require.NoError(t, err)

	retried, err := ts.Service.RetryDocument(userCtx, &v1pb.RetryDocumentRequest{Name: document.Name})
	require.NoError(t, err)
	require.Equal(t, v1pb.DocumentStatus_DOCUMENT_STATUS_FAILED, retried.Status)
	require.Equal(t, "SOURCE_HASH_MISMATCH", retried.ErrorCode)
	require.Equal(t, originalHash, retried.Sha256)

	documentUID := document.Name[len("documents/"):]
	storedDocument, err := ts.Store.GetDocument(ctx, &store.FindDocument{UID: &documentUID, CreatorID: user.ID})
	require.NoError(t, err)
	derived, err := ts.Store.GetDocumentContent(ctx, storedDocument.ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, "original", derived.PlainText, "integrity failure must preserve the last successful output")
}

func TestDocumentParsingExplicitOutcomes(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()
	ctx := context.Background()
	user, err := ts.CreateRegularUser(ctx, "document-outcomes")
	require.NoError(t, err)
	userCtx := ts.CreateUserContext(ctx, user.ID)

	tests := []struct {
		name       string
		filename   string
		mimeType   string
		content    []byte
		wantStatus v1pb.DocumentStatus
		wantCode   string
	}{
		{name: "invalid UTF-8", filename: "invalid.txt", mimeType: "text/plain", content: []byte{0xff}, wantStatus: v1pb.DocumentStatus_DOCUMENT_STATUS_FAILED, wantCode: "INVALID_UTF8"},
		{name: "binary text", filename: "binary.txt", mimeType: "text/plain", content: []byte{'a', 0, 'b'}, wantStatus: v1pb.DocumentStatus_DOCUMENT_STATUS_FAILED, wantCode: "BINARY_CONTENT"},
		{name: "parser input too large", filename: "large.txt", mimeType: "text/plain", content: bytes.Repeat([]byte{'a'}, documentcore.DefaultMaxInputBytes+1), wantStatus: v1pb.DocumentStatus_DOCUMENT_STATUS_FAILED, wantCode: "INPUT_TOO_LARGE"},
		{name: "malformed modern Office file", filename: "broken.docx", mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", content: []byte("PKbroken"), wantStatus: v1pb.DocumentStatus_DOCUMENT_STATUS_FAILED, wantCode: "INVALID_OFFICE_DOCUMENT"},
		{name: "legacy parser not implemented", filename: "legacy.doc", mimeType: "application/msword", content: append([]byte{}, 0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1), wantStatus: v1pb.DocumentStatus_DOCUMENT_STATUS_UNSUPPORTED, wantCode: "UNSUPPORTED_FORMAT"},
		{name: "legacy spreadsheet not implemented", filename: "legacy.xls", mimeType: "application/vnd.ms-excel", content: append([]byte{}, 0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1), wantStatus: v1pb.DocumentStatus_DOCUMENT_STATUS_UNSUPPORTED, wantCode: "UNSUPPORTED_FORMAT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attachment, err := ts.Service.CreateAttachment(userCtx, &v1pb.CreateAttachmentRequest{Attachment: &v1pb.Attachment{
				Filename: test.filename, Type: test.mimeType, Content: test.content,
			}})
			require.NoError(t, err)
			document, err := ts.Service.CreateDocument(userCtx, &v1pb.CreateDocumentRequest{Attachment: attachment.Name})
			require.NoError(t, err)
			require.Equal(t, test.wantStatus, document.Status)
			require.Equal(t, test.wantCode, document.ErrorCode)
			if test.wantStatus == v1pb.DocumentStatus_DOCUMENT_STATUS_UNSUPPORTED {
				require.Contains(t, document.ErrorMessage, "旧版 Office 文件")
			}
		})
	}
}

func TestDocumentServiceParsesPDFWithPageLocators(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()
	ctx := context.Background()
	user, err := ts.CreateRegularUser(ctx, "document-pdf")
	require.NoError(t, err)
	userCtx := ts.CreateUserContext(ctx, user.ID)
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "core", "document", "testdata", "two-page-text.pdf"))
	require.NoError(t, err)

	attachment, err := ts.Service.CreateAttachment(userCtx, &v1pb.CreateAttachmentRequest{Attachment: &v1pb.Attachment{
		Filename: "learning.pdf", Type: "application/pdf", Content: content,
	}})
	require.NoError(t, err)
	document, err := ts.Service.CreateDocument(userCtx, &v1pb.CreateDocumentRequest{Attachment: attachment.Name})
	require.NoError(t, err)
	require.Equal(t, v1pb.DocumentStatus_DOCUMENT_STATUS_READY, document.Status)
	require.Equal(t, documentcore.PDFParserVersion, document.ParserVersion)

	documentUID := document.Name[len("documents/"):]
	storedDocument, err := ts.Store.GetDocument(ctx, &store.FindDocument{UID: &documentUID, CreatorID: user.ID})
	require.NoError(t, err)
	derived, err := ts.Store.GetDocumentContent(ctx, storedDocument.ID, user.ID)
	require.NoError(t, err)
	require.Contains(t, derived.PlainText, "Page one: source-linked knowledge.")
	require.Contains(t, derived.StructuredJSON, `"pageCount":2`)
	require.Contains(t, derived.StructuredJSON, `"page":2`)
}

func TestDocumentServiceParsesModernOfficeDocuments(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()
	ctx := context.Background()
	user, err := ts.CreateRegularUser(ctx, "document-office")
	require.NoError(t, err)
	userCtx := ts.CreateUserContext(ctx, user.ID)

	tests := []struct {
		filename  string
		mimeType  string
		content   []byte
		extractor string
		plainText string
		locator   string
	}{
		{
			filename: "notes.docx",
			mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			content: officeServiceTestArchive(t, map[string]string{
				"[Content_Types].xml": `<Types/>`,
				"word/document.xml":   `<w:document xmlns:w="urn:w"><w:body><w:p><w:r><w:t>Office knowledge</w:t></w:r></w:p></w:body></w:document>`,
			}),
			extractor: documentcore.DOCXParserVersion,
			plainText: "Office knowledge",
			locator:   `"locator":"paragraph:1"`,
		},
		{
			filename: "sources.xlsx",
			mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			content: officeServiceTestArchive(t, map[string]string{
				"[Content_Types].xml":        `<Types/>`,
				"xl/workbook.xml":            `<workbook xmlns:r="urn:r"><sheets><sheet name="Sources" r:id="r1"/></sheets></workbook>`,
				"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="r1" Target="worksheets/sheet1.xml"/></Relationships>`,
				"xl/worksheets/sheet1.xml":   `<worksheet><sheetData><row><c r="B2" t="inlineStr"><is><t>Primary source</t></is></c></row></sheetData></worksheet>`,
			}),
			extractor: documentcore.XLSXParserVersion,
			plainText: "B2: Primary source",
			locator:   `"reference":"B2"`,
		},
	}

	for _, test := range tests {
		t.Run(test.filename, func(t *testing.T) {
			attachment, err := ts.Service.CreateAttachment(userCtx, &v1pb.CreateAttachmentRequest{Attachment: &v1pb.Attachment{
				Filename: test.filename, Type: test.mimeType, Content: test.content,
			}})
			require.NoError(t, err)
			document, err := ts.Service.CreateDocument(userCtx, &v1pb.CreateDocumentRequest{Attachment: attachment.Name})
			require.NoError(t, err)
			require.Equal(t, v1pb.DocumentStatus_DOCUMENT_STATUS_READY, document.Status)
			require.Equal(t, test.extractor, document.ParserVersion)

			documentUID := document.Name[len("documents/"):]
			storedDocument, err := ts.Store.GetDocument(ctx, &store.FindDocument{UID: &documentUID, CreatorID: user.ID})
			require.NoError(t, err)
			derived, err := ts.Store.GetDocumentContent(ctx, storedDocument.ID, user.ID)
			require.NoError(t, err)
			require.Contains(t, derived.PlainText, test.plainText)
			require.Contains(t, derived.StructuredJSON, test.locator)
		})
	}
}

func TestDocumentServiceRequiresAuthentication(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()
	ctx := context.Background()

	calls := []func() error{
		func() error { _, err := ts.Service.CreateDocument(ctx, &v1pb.CreateDocumentRequest{}); return err },
		func() error { _, err := ts.Service.ListDocuments(ctx, &v1pb.ListDocumentsRequest{}); return err },
		func() error { _, err := ts.Service.GetDocument(ctx, &v1pb.GetDocumentRequest{}); return err },
		func() error { _, err := ts.Service.RetryDocument(ctx, &v1pb.RetryDocumentRequest{}); return err },
		func() error {
			_, err := ts.Service.GetDocumentDeletePlan(ctx, &v1pb.GetDocumentDeletePlanRequest{})
			return err
		},
		func() error { _, err := ts.Service.DeleteDocument(ctx, &v1pb.DeleteDocumentRequest{}); return err },
	}
	for _, call := range calls {
		require.Equal(t, codes.Unauthenticated, status.Code(call()))
	}
}

func TestDocumentServiceValidatesFileMetadataAndContent(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()
	ctx := context.Background()
	user, err := ts.CreateRegularUser(ctx, "document-validation")
	require.NoError(t, err)
	userCtx := ts.CreateUserContext(ctx, user.ID)

	tests := []struct {
		name     string
		filename string
		mimeType string
		content  []byte
	}{
		{name: "unsupported extension", filename: "notes.exe", mimeType: "application/octet-stream", content: []byte("no")},
		{name: "mismatched content type", filename: "notes.pdf", mimeType: "text/plain", content: []byte("%PDF-1.7")},
		{name: "invalid PDF signature", filename: "notes.pdf", mimeType: "application/pdf", content: []byte("not a pdf")},
		{name: "invalid DOCX signature", filename: "notes.docx", mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", content: []byte("not zip")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attachment, err := ts.Service.CreateAttachment(userCtx, &v1pb.CreateAttachmentRequest{Attachment: &v1pb.Attachment{
				Filename: test.filename,
				Type:     test.mimeType,
				Content:  test.content,
			}})
			require.NoError(t, err)
			_, err = ts.Service.CreateDocument(userCtx, &v1pb.CreateDocumentRequest{Attachment: attachment.Name})
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}

	_, err = ts.Store.UpsertInstanceSetting(ctx, &storepb.InstanceSetting{
		Key: storepb.InstanceSettingKey_STORAGE,
		Value: &storepb.InstanceSetting_StorageSetting{StorageSetting: &storepb.InstanceStorageSetting{
			UploadSizeLimitMb: 1,
		}},
	})
	require.NoError(t, err)
	oversized, err := ts.Store.CreateAttachment(ctx, &store.Attachment{
		UID: shortuuid.New(), CreatorID: user.ID, Filename: "large.txt", Type: "text/plain", Size: 2 << 20, Blob: []byte("small"),
	})
	require.NoError(t, err)
	_, err = ts.Service.CreateDocument(userCtx, &v1pb.CreateDocumentRequest{Attachment: "attachments/" + oversized.UID})
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}

func officeServiceTestArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = entry.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}
