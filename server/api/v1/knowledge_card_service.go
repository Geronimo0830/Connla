package v1

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/lithammer/shortuuid/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

// CreateKnowledgeCard saves one manually written card, optionally tied to an exact extracted-text range.
func (s *APIV1Service) CreateKnowledgeCard(ctx context.Context, req *v1pb.CreateKnowledgeCardRequest) (*v1pb.KnowledgeCard, error) {
	user, err := s.requireDocumentUser(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetCard() == nil {
		return nil, status.Error(codes.InvalidArgument, "card is required")
	}
	card, err := s.cardFromRequest(ctx, req.Card, user.ID)
	if err != nil {
		return nil, err
	}
	card.UID = shortuuid.New()
	if req.Source != nil {
		selection := req.Source
		doc, err := s.getOwnedDocument(ctx, selection.GetDocument(), user.ID)
		if err != nil {
			return nil, err
		}
		if doc.Status != store.DocumentStatusReady {
			return nil, status.Error(codes.FailedPrecondition, "source document is not ready")
		}
		content, err := s.Store.GetDocumentContent(ctx, doc.ID, user.ID)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to read source content")
		}
		if content == nil || content.ContentHash != selection.GetContentHash() {
			return nil, status.Error(codes.FailedPrecondition, "source text changed; select it again")
		}
		text := []rune(content.PlainText)
		start, end := int(selection.GetStartOffset()), int(selection.GetEndOffset())
		if start < 0 || end <= start || end > len(text) || end-start > 2000 {
			return nil, status.Error(codes.InvalidArgument, "invalid source selection")
		}
		card.SourceDocumentID = &doc.ID
		card.SourceDocumentUID = doc.UID
		card.SourceTitle = doc.Title
		card.SourceQuote = string(text[start:end])
		card.SourceLocator = "text:" + strconv.Itoa(start) + ":" + strconv.Itoa(end)
		card.SourceContentHash = content.ContentHash
	}
	saved, err := s.Store.CreateKnowledgeCard(ctx, card)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "card could not be saved")
	}
	return s.convertKnowledgeCard(ctx, saved)
}

// ListKnowledgeCards returns only the signed-in user's cards.
func (s *APIV1Service) ListKnowledgeCards(ctx context.Context, req *v1pb.ListKnowledgeCardsRequest) (*v1pb.ListKnowledgeCardsResponse, error) {
	user, err := s.requireDocumentUser(ctx)
	if err != nil {
		return nil, err
	}
	cards, err := s.Store.ListKnowledgeCards(ctx, user.ID, req.GetIncludeArchived())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list cards")
	}
	out := &v1pb.ListKnowledgeCardsResponse{}
	for _, card := range cards {
		item, err := s.convertKnowledgeCard(ctx, card)
		if err != nil {
			return nil, err
		}
		out.Cards = append(out.Cards, item)
	}
	return out, nil
}

// UpdateKnowledgeCard changes writing, kind, topics, or archive state without altering source evidence.
func (s *APIV1Service) UpdateKnowledgeCard(ctx context.Context, req *v1pb.UpdateKnowledgeCardRequest) (*v1pb.KnowledgeCard, error) {
	user, err := s.requireDocumentUser(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetCard() == nil {
		return nil, status.Error(codes.InvalidArgument, "card is required")
	}
	uid, err := extractKnowledgeCardUID(req.Card.GetName())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid card name")
	}
	card, err := s.Store.GetKnowledgeCard(ctx, uid, user.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read card")
	}
	if card == nil {
		return nil, status.Error(codes.NotFound, "card not found")
	}
	editable, err := s.cardFromRequest(ctx, req.Card, user.ID)
	if err != nil {
		return nil, err
	}
	card.Title, card.Body, card.CardType = editable.Title, editable.Body, editable.CardType
	card.TopicIDs, card.Archived = editable.TopicIDs, req.Card.GetArchived()
	saved, err := s.Store.UpdateKnowledgeCard(ctx, card)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "card could not be updated")
	}
	return s.convertKnowledgeCard(ctx, saved)
}

func (s *APIV1Service) cardFromRequest(ctx context.Context, input *v1pb.KnowledgeCard, ownerID int32) (*store.KnowledgeCard, error) {
	kind := input.GetCardType()
	if kind == "" {
		kind = "idea"
	}
	card := &store.KnowledgeCard{CreatorID: ownerID, Title: strings.TrimSpace(input.GetTitle()),
		Body: strings.TrimSpace(input.GetBody()), CardType: kind, Archived: input.GetArchived()}
	if card.Title == "" || card.Body == "" || !store.ValidKnowledgeCardType(card.CardType) ||
		len([]rune(card.Title)) > 200 || len([]rune(card.Body)) > 20000 || len(input.GetTopics()) > 30 {
		return nil, status.Error(codes.InvalidArgument, "card title, body, type, or topics are invalid")
	}
	seen := make(map[int32]bool)
	for _, name := range input.GetTopics() {
		topic, err := s.getOwnedTopic(ctx, name, ownerID)
		if err != nil {
			return nil, err
		}
		if !seen[topic.ID] {
			card.TopicIDs = append(card.TopicIDs, topic.ID)
			seen[topic.ID] = true
		}
	}
	return card, nil
}

func (s *APIV1Service) convertKnowledgeCard(ctx context.Context, card *store.KnowledgeCard) (*v1pb.KnowledgeCard, error) {
	out := &v1pb.KnowledgeCard{Name: "knowledge/cards/" + card.UID, Title: card.Title, Body: card.Body,
		CardType: card.CardType, Archived: card.Archived, SourceTitle: card.SourceTitle,
		SourceQuote: card.SourceQuote, SourceLocator: card.SourceLocator, SourceContentHash: card.SourceContentHash,
		CreateTime: timestamppb.New(time.Unix(card.CreatedTs, 0)), UpdateTime: timestamppb.New(time.Unix(card.UpdatedTs, 0))}
	if card.SourceDocumentUID != "" {
		out.SourceDocument = "documents/" + card.SourceDocumentUID
		out.SourceAvailable = card.SourceDocumentID != nil
	}
	for _, id := range card.TopicIDs {
		topic, err := s.Store.GetKnowledgeTopic(ctx, &store.FindKnowledgeTopic{ID: &id, CreatorID: card.CreatorID})
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to read card topic")
		}
		if topic != nil {
			out.Topics = append(out.Topics, "knowledge/topics/"+topic.UID)
		}
	}
	return out, nil
}

func extractKnowledgeCardUID(name string) (string, error) {
	const prefix = "knowledge/cards/"
	if !strings.HasPrefix(name, prefix) || strings.TrimPrefix(name, prefix) == "" || strings.Contains(strings.TrimPrefix(name, prefix), "/") {
		return "", status.Error(codes.InvalidArgument, "invalid card name")
	}
	return strings.TrimPrefix(name, prefix), nil
}
