package v1

import (
	"strings"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

func documentName(uid string) string {
	return "documents/" + uid
}

// ExtractDocumentUIDFromName parses a Document resource name.
func ExtractDocumentUIDFromName(name string) (string, error) {
	uid := strings.TrimPrefix(name, "documents/")
	if uid == "" || uid == name || strings.Contains(uid, "/") {
		return "", errors.New("invalid document name")
	}
	return uid, nil
}

func convertDocumentFromStore(document *store.Document) *v1pb.Document {
	if document == nil {
		return nil
	}
	result := &v1pb.Document{
		Name:             documentName(document.UID),
		SourceAttachment: "attachments/" + document.AttachmentUID,
		CreateTime:       timestamppb.New(time.Unix(document.CreatedTs, 0)),
		UpdateTime:       timestamppb.New(time.Unix(document.UpdatedTs, 0)),
		Title:            document.Title,
		OriginalFilename: document.OriginalFilename,
		Extension:        document.Extension,
		MediaType:        document.MediaType,
		Size:             document.ByteSize,
		Sha256:           document.SHA256,
		Status:           convertDocumentStatusFromStore(document.Status),
		ParserVersion:    document.ParserVersion,
		ErrorCode:        document.ErrorCode,
		ErrorMessage:     document.ErrorMessage,
	}
	if document.ParsedTs != nil {
		result.ParsedTime = timestamppb.New(time.Unix(*document.ParsedTs, 0))
	}
	return result
}

func convertDocumentStatusFromStore(status store.DocumentStatus) v1pb.DocumentStatus {
	switch status {
	case store.DocumentStatusUploaded:
		return v1pb.DocumentStatus_DOCUMENT_STATUS_UPLOADED
	case store.DocumentStatusQueued:
		return v1pb.DocumentStatus_DOCUMENT_STATUS_QUEUED
	case store.DocumentStatusParsing:
		return v1pb.DocumentStatus_DOCUMENT_STATUS_PARSING
	case store.DocumentStatusReady:
		return v1pb.DocumentStatus_DOCUMENT_STATUS_READY
	case store.DocumentStatusFailed:
		return v1pb.DocumentStatus_DOCUMENT_STATUS_FAILED
	case store.DocumentStatusUnsupported:
		return v1pb.DocumentStatus_DOCUMENT_STATUS_UNSUPPORTED
	default:
		return v1pb.DocumentStatus_DOCUMENT_STATUS_UNSPECIFIED
	}
}
