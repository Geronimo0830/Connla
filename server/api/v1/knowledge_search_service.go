package v1

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

// SearchKnowledge searches the authenticated user's local document projection.
func (s *APIV1Service) SearchKnowledge(ctx context.Context, req *v1pb.SearchKnowledgeRequest) (*v1pb.SearchKnowledgeResponse, error) {
	user, err := s.requireDocumentUser(ctx)
	if err != nil {
		return nil, err
	}
	query := strings.TrimSpace(req.GetQuery())
	if query == "" {
		return nil, status.Error(codes.InvalidArgument, "search query is required")
	}
	if len([]rune(query)) > 200 {
		return nil, status.Error(codes.InvalidArgument, "search query is too long")
	}
	contentType := req.GetContentType()
	if contentType != "" && contentType != "document" && contentType != "card" {
		return nil, status.Error(codes.InvalidArgument, "invalid content type")
	}
	fileFormat := strings.ToLower(strings.TrimSpace(req.GetFileFormat()))
	if fileFormat != "" && !isKnowledgeSearchFormat(fileFormat) {
		return nil, status.Error(codes.InvalidArgument, "invalid file format")
	}
	if contentType == "card" && fileFormat != "" {
		return nil, status.Error(codes.InvalidArgument, "file format is only available for documents")
	}
	if req.GetCreatedFrom() < 0 || req.GetCreatedBefore() < 0 ||
		(req.GetCreatedFrom() > 0 && req.GetCreatedBefore() > 0 && req.GetCreatedFrom() >= req.GetCreatedBefore()) {
		return nil, status.Error(codes.InvalidArgument, "invalid creation date range")
	}
	var topicID *int32
	if req.GetTopic() != "" {
		topic, err := s.getOwnedTopic(ctx, req.GetTopic(), user.ID)
		if err != nil {
			return nil, err
		}
		topicID = &topic.ID
	}
	filters := store.KnowledgeSearchFilters{TopicID: topicID, FileFormat: fileFormat, CreatedFrom: req.GetCreatedFrom(), CreatedBefore: req.GetCreatedBefore()}
	out := &v1pb.SearchKnowledgeResponse{}
	if contentType != "card" {
		results, err := s.Store.SearchKnowledgeDocuments(ctx, user.ID, query, filters, int(req.GetPageSize()))
		if errors.Is(err, store.ErrKnowledgeSearchUnsupported) {
			return nil, status.Error(codes.Unimplemented, "offline full-text search is available only with SQLite")
		}
		if err != nil {
			return nil, status.Error(codes.Internal, "knowledge search failed")
		}
		for _, r := range results {
			out.Results = append(out.Results, &v1pb.KnowledgeSearchResult{Document: "documents/" + r.DocumentUID, Title: r.Title, OriginalFilename: r.OriginalFilename, Snippet: r.Snippet})
		}
	}
	if contentType != "document" && fileFormat == "" {
		cards, err := s.Store.SearchKnowledgeCards(ctx, user.ID, query, filters, int(req.GetPageSize()))
		if errors.Is(err, store.ErrKnowledgeSearchUnsupported) {
			return nil, status.Error(codes.Unimplemented, "offline full-text search is available only with SQLite")
		}
		if err != nil {
			return nil, status.Error(codes.Internal, "knowledge card search failed")
		}
		for _, r := range cards {
			out.Results = append(out.Results, &v1pb.KnowledgeSearchResult{Card: "knowledge/cards/" + r.CardUID, Title: r.Title, Snippet: r.Snippet})
		}
	}
	return out, nil
}

func isKnowledgeSearchFormat(format string) bool {
	switch format {
	case ".txt", ".md", ".markdown", ".pdf", ".doc", ".docx", ".xls", ".xlsx":
		return true
	default:
		return false
	}
}

// RebuildKnowledgeSearch repairs the authenticated user's replaceable search projection.
func (s *APIV1Service) RebuildKnowledgeSearch(ctx context.Context, _ *v1pb.RebuildKnowledgeSearchRequest) (*v1pb.RebuildKnowledgeSearchResponse, error) {
	user, err := s.requireDocumentUser(ctx)
	if err != nil {
		return nil, err
	}
	count, err := s.Store.RebuildKnowledgeSearch(ctx, user.ID)
	if errors.Is(err, store.ErrKnowledgeSearchUnsupported) {
		return nil, status.Error(codes.Unimplemented, "offline full-text search is available only with SQLite")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "knowledge search rebuild failed")
	}
	cards, err := s.Store.CountIndexedKnowledgeCards(ctx, user.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "knowledge search count failed")
	}
	return &v1pb.RebuildKnowledgeSearchResponse{IndexedDocuments: count, IndexedCards: cards}, nil
}
