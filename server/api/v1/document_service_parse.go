package v1

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/pkg/errors"

	documentcore "github.com/usememos/memos/core/document"
	"github.com/usememos/memos/store"
)

const (
	documentParseTimeout        = 10 * time.Second
	documentFailureWriteTimeout = 5 * time.Second
)

func (s *APIV1Service) parseDocument(ctx context.Context, document *store.Document) (*store.Document, error) {
	parserVersion := documentcore.ParserVersion(document.Extension)
	attempt, err := s.Store.BeginDocumentParse(ctx, document.ID, document.CreatorID, parserVersion)
	if err != nil {
		return nil, errors.Wrap(err, "failed to begin document parse")
	}
	if attempt == 0 {
		return nil, errors.New("document not found")
	}
	started := time.Now()
	parseCtx, cancel := context.WithTimeout(ctx, documentParseTimeout)
	defer cancel()

	attachment, err := s.Store.GetAttachment(parseCtx, &store.FindAttachment{ID: &document.AttachmentID})
	if err == nil && attachment == nil {
		err = errors.New("source attachment not found")
	}
	var blob []byte
	if err == nil {
		blob, err = s.GetAttachmentBlob(parseCtx, attachment)
	}
	if err != nil {
		return s.recordDocumentParseFailure(ctx, document, attempt, parserVersion, "SOURCE_READ_FAILED", "source file could not be read", started, false)
	}
	sourceHash := sha256.Sum256(blob)
	if hex.EncodeToString(sourceHash[:]) != document.SHA256 {
		return s.recordDocumentParseFailure(ctx, document, attempt, parserVersion, "SOURCE_HASH_MISMATCH", "source file integrity check failed", started, false)
	}

	parser := s.DocumentParser
	if parser == nil {
		parser = documentcore.NewParser(s.MarkdownService, documentcore.DefaultMaxInputBytes)
	}
	result, parseErr := parser.Parse(parseCtx, document.Extension, document.MediaType, blob)
	if parseErr != nil {
		unsupported := errors.Is(parseErr, documentcore.ErrUnsupportedFormat)
		return s.recordDocumentParseFailure(
			ctx, document, attempt, parserVersion, documentcore.ErrorCode(parseErr), safeDocumentParseMessage(parseErr, document.Extension), started, unsupported,
		)
	}
	durationMs := max(time.Since(started).Milliseconds(), int64(0))
	if err := s.Store.CompleteDocumentParse(ctx, document.ID, document.CreatorID, attempt, &store.DocumentContent{
		DocumentID:     document.ID,
		PlainText:      result.PlainText,
		StructuredJSON: result.StructuredJSON,
		ContentHash:    result.ContentHash,
		Extractor:      result.Extractor,
	}, parserVersion, durationMs); err != nil {
		return nil, errors.Wrap(err, "failed to complete document parse")
	}
	return s.Store.GetDocument(ctx, &store.FindDocument{ID: &document.ID, CreatorID: document.CreatorID})
}

func (s *APIV1Service) recordDocumentParseFailure(
	ctx context.Context,
	document *store.Document,
	attempt int32,
	parserVersion, errorCode, safeMessage string,
	started time.Time,
	unsupported bool,
) (*store.Document, error) {
	result := store.DocumentParseResultFailed
	if unsupported {
		result = store.DocumentParseResultUnsupported
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), documentFailureWriteTimeout)
	defer cancel()
	durationMs := max(time.Since(started).Milliseconds(), int64(0))
	if err := s.Store.FailDocumentParse(
		writeCtx, document.ID, document.CreatorID, attempt, result, parserVersion, errorCode, safeMessage, durationMs,
	); err != nil {
		return nil, errors.Wrap(err, "failed to persist document parse failure")
	}
	updated, err := s.Store.GetDocument(writeCtx, &store.FindDocument{ID: &document.ID, CreatorID: document.CreatorID})
	if err != nil {
		return nil, errors.Wrap(err, "failed to reload failed document")
	}
	return updated, nil
}

func safeDocumentParseMessage(err error, extension string) string {
	switch documentcore.ErrorCode(err) {
	case "UNSUPPORTED_FORMAT":
		if extension == ".doc" || extension == ".xls" {
			return "这是旧版 Office 文件，暂不支持内容识别。请转换为 .docx 或 .xlsx 后重新导入；原文件仍可下载。"
		}
		return "this document format is not supported yet"
	case "INVALID_UTF8":
		return "the document is not valid UTF-8 text"
	case "BINARY_CONTENT":
		return "the text document contains binary data"
	case "INPUT_TOO_LARGE":
		return "the document exceeds the parser size limit"
	case "INVALID_PDF":
		return "the PDF is malformed or cannot be read"
	case "PDF_PAGE_LIMIT":
		return "the PDF exceeds the 500-page limit"
	case "PDF_TEXT_TOO_LARGE":
		return "the PDF contains too much extracted text"
	case "INVALID_OFFICE_DOCUMENT":
		return "the Office document is malformed or cannot be read"
	case "OFFICE_ARCHIVE_LIMIT":
		return "the Office document exceeds safe archive limits"
	case "OFFICE_CONTENT_LIMIT":
		return "the Office document contains too much content"
	case "TIMEOUT":
		return "document parsing timed out"
	default:
		return "document parsing failed"
	}
}
