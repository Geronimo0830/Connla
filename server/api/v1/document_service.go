package v1

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lithammer/shortuuid/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

const (
	defaultDocumentPageSize = 50
	maxDocumentPageSize     = 100
)

var documentMediaTypes = map[string]map[string]bool{
	".txt":      {"text/plain": true},
	".md":       {"text/markdown": true, "text/plain": true},
	".markdown": {"text/markdown": true, "text/plain": true},
	".pdf":      {"application/pdf": true},
	".doc":      {"application/msword": true},
	".docx":     {"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true},
	".xls":      {"application/vnd.ms-excel": true},
	".xlsx":     {"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true},
}

var (
	zipMagic = []byte{'P', 'K'}
	oleMagic = []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}
)

// CreateDocument registers an authenticated user's existing attachment as an immutable document source.
func (s *APIV1Service) CreateDocument(ctx context.Context, request *v1pb.CreateDocumentRequest) (*v1pb.Document, error) {
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}
	attachmentUID, err := ExtractAttachmentUIDFromName(request.GetAttachment())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid attachment: %v", err)
	}
	attachment, err := s.Store.GetAttachment(ctx, &store.FindAttachment{UID: &attachmentUID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get attachment: %v", err)
	}
	if attachment == nil {
		return nil, status.Error(codes.NotFound, "attachment not found")
	}
	if attachment.CreatorID != user.ID {
		return nil, status.Error(codes.PermissionDenied, "permission denied")
	}
	if existing, err := s.Store.GetDocument(ctx, &store.FindDocument{AttachmentID: &attachment.ID, CreatorID: user.ID}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check document: %v", err)
	} else if existing != nil {
		return nil, status.Error(codes.AlreadyExists, "attachment is already registered as a document")
	}

	extension := strings.ToLower(filepath.Ext(attachment.Filename))
	if err := validateDocumentMetadata(extension, attachment.Type, attachment.Size); err != nil {
		return nil, err
	}
	instanceStorageSetting, err := s.Store.GetInstanceStorageSetting(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get instance storage setting: %v", err)
	}
	if err := checkUploadSize(instanceStorageSetting, attachment.Size); err != nil {
		return nil, err
	}
	blob, err := s.GetAttachmentBlob(ctx, attachment)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read attachment: %v", err)
	}
	if int64(len(blob)) != attachment.Size {
		return nil, status.Error(codes.FailedPrecondition, "attachment size does not match stored content")
	}
	if err := validateDocumentSignature(extension, blob); err != nil {
		return nil, err
	}
	hash := sha256.Sum256(blob)
	title := strings.TrimSpace(request.GetTitle())
	if title == "" {
		title = attachment.Filename
	}
	document, err := s.Store.CreateDocument(ctx, &store.Document{
		UID:              shortuuid.New(),
		CreatorID:        user.ID,
		AttachmentID:     attachment.ID,
		Title:            title,
		OriginalFilename: attachment.Filename,
		Extension:        extension,
		MediaType:        attachment.Type,
		ByteSize:         attachment.Size,
		SHA256:           hex.EncodeToString(hash[:]),
		Status:           store.DocumentStatusUploaded,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create document: %v", err)
	}
	document, err = s.parseDocument(ctx, document)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to process document: %v", err)
	}
	return convertDocumentFromStore(document), nil
}

// ListDocuments lists only documents owned by the authenticated user.
func (s *APIV1Service) ListDocuments(ctx context.Context, request *v1pb.ListDocumentsRequest) (*v1pb.ListDocumentsResponse, error) {
	user, err := s.requireDocumentUser(ctx)
	if err != nil {
		return nil, err
	}
	pageSize := int(request.GetPageSize())
	if pageSize <= 0 {
		pageSize = defaultDocumentPageSize
	}
	pageSize = min(pageSize, maxDocumentPageSize)
	offset := 0
	if request.GetPageToken() != "" {
		offset, err = strconv.Atoi(request.GetPageToken())
		if err != nil || offset < 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid page token")
		}
	}
	limit := pageSize + 1
	documents, err := s.Store.ListDocuments(ctx, &store.FindDocument{CreatorID: user.ID, Limit: &limit, Offset: &offset})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list documents: %v", err)
	}
	response := &v1pb.ListDocumentsResponse{}
	for _, document := range documents[:min(len(documents), pageSize)] {
		response.Documents = append(response.Documents, convertDocumentFromStore(document))
	}
	if len(documents) > pageSize {
		response.NextPageToken = strconv.Itoa(offset + pageSize)
	}
	return response, nil
}

// GetDocument returns one document only inside the authenticated user's owner boundary.
func (s *APIV1Service) GetDocument(ctx context.Context, request *v1pb.GetDocumentRequest) (*v1pb.Document, error) {
	user, err := s.requireDocumentUser(ctx)
	if err != nil {
		return nil, err
	}
	document, err := s.getOwnedDocument(ctx, request.GetName(), user.ID)
	if err != nil {
		return nil, err
	}
	return convertDocumentFromStore(document), nil
}

// GetDocumentContent returns only the owner's replaceable extracted text for reading and selection.
func (s *APIV1Service) GetDocumentContent(ctx context.Context, request *v1pb.GetDocumentContentRequest) (*v1pb.DocumentContent, error) {
	user, err := s.requireDocumentUser(ctx)
	if err != nil {
		return nil, err
	}
	document, err := s.getOwnedDocument(ctx, request.GetName(), user.ID)
	if err != nil {
		return nil, err
	}
	content, err := s.Store.GetDocumentContent(ctx, document.ID, user.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read document content")
	}
	if content == nil {
		return nil, status.Error(codes.FailedPrecondition, "document has no extracted text")
	}
	return &v1pb.DocumentContent{Document: request.GetName(), PlainText: content.PlainText,
		StructuredJson: content.StructuredJSON, ContentHash: content.ContentHash}, nil
}

// RetryDocument queues an owned document for a future parser worker.
func (s *APIV1Service) RetryDocument(ctx context.Context, request *v1pb.RetryDocumentRequest) (*v1pb.Document, error) {
	user, err := s.requireDocumentUser(ctx)
	if err != nil {
		return nil, err
	}
	document, err := s.getOwnedDocument(ctx, request.GetName(), user.ID)
	if err != nil {
		return nil, err
	}
	document, err = s.parseDocument(ctx, document)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "document cannot be retried: %v", err)
	}
	return convertDocumentFromStore(document), nil
}

// GetDocumentDeletePlan previews exactly what a confirmed deletion will remove.
func (s *APIV1Service) GetDocumentDeletePlan(ctx context.Context, request *v1pb.GetDocumentDeletePlanRequest) (*v1pb.DocumentDeletePlan, error) {
	user, err := s.requireDocumentUser(ctx)
	if err != nil {
		return nil, err
	}
	document, err := s.getOwnedDocument(ctx, request.GetName(), user.ID)
	if err != nil {
		return nil, err
	}
	contentRows, attemptRows, found, err := s.Store.GetDocumentDeleteImpact(ctx, document.ID, user.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to build delete plan: %v", err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "document not found")
	}
	linkedCards, err := s.Store.CountDocumentKnowledgeCards(ctx, document.ID, user.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to count linked cards")
	}
	return &v1pb.DocumentDeletePlan{
		Name:                     documentName(document.UID),
		SourceAttachment:         "attachments/" + document.AttachmentUID,
		SourceAttachmentRetained: true,
		DerivedContentRows:       contentRows,
		ParseAttemptRows:         attemptRows,
		LinkedCardCount:          linkedCards,
	}, nil
}

// DeleteDocument requires explicit confirmation and retains the source attachment.
func (s *APIV1Service) DeleteDocument(ctx context.Context, request *v1pb.DeleteDocumentRequest) (*emptypb.Empty, error) {
	user, err := s.requireDocumentUser(ctx)
	if err != nil {
		return nil, err
	}
	if !request.GetConfirm() {
		return nil, status.Error(codes.FailedPrecondition, "document deletion requires explicit confirmation")
	}
	document, err := s.getOwnedDocument(ctx, request.GetName(), user.ID)
	if err != nil {
		return nil, err
	}
	found, err := s.Store.DeleteOwnedDocument(ctx, document.ID, user.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete document: %v", err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "document not found")
	}
	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) requireDocumentUser(ctx context.Context) (*store.User, error) {
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}
	return user, nil
}

func (s *APIV1Service) getOwnedDocument(ctx context.Context, name string, creatorID int32) (*store.Document, error) {
	uid, err := ExtractDocumentUIDFromName(name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid document name: %v", err)
	}
	document, err := s.Store.GetDocument(ctx, &store.FindDocument{UID: &uid, CreatorID: creatorID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get document: %v", err)
	}
	if document == nil {
		return nil, status.Error(codes.NotFound, "document not found")
	}
	return document, nil
}

func validateDocumentMetadata(extension, mediaType string, size int64) error {
	allowedTypes, ok := documentMediaTypes[extension]
	if !ok {
		return status.Errorf(codes.InvalidArgument, "unsupported document extension: %s", extension)
	}
	if !allowedTypes[mediaType] {
		return status.Errorf(codes.InvalidArgument, "content type %s does not match extension %s", mediaType, extension)
	}
	if size < 0 {
		return status.Error(codes.InvalidArgument, "document size is invalid")
	}
	return nil
}

func validateDocumentSignature(extension string, blob []byte) error {
	switch extension {
	case ".pdf":
		if !bytes.HasPrefix(blob, []byte("%PDF-")) {
			return status.Error(codes.InvalidArgument, "file content is not a PDF")
		}
	case ".docx", ".xlsx":
		if !bytes.HasPrefix(blob, zipMagic) {
			return status.Error(codes.InvalidArgument, "file content is not an Office Open XML document")
		}
	case ".doc", ".xls":
		if !bytes.HasPrefix(blob, oleMagic) {
			return status.Error(codes.InvalidArgument, "file content is not a legacy Office document")
		}
	default:
	}
	return nil
}
