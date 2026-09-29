package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

func TestKnowledgeCardSourceOwnershipSearchAndArchive(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()
	ctx := context.Background()
	owner, err := ts.CreateRegularUser(ctx, "card-owner")
	require.NoError(t, err)
	other, err := ts.CreateRegularUser(ctx, "card-other")
	require.NoError(t, err)
	ownerCtx := ts.CreateUserContext(ctx, owner.ID)
	otherCtx := ts.CreateUserContext(ctx, other.ID)

	attachment, err := ts.Service.CreateAttachment(ownerCtx, &v1pb.CreateAttachmentRequest{Attachment: &v1pb.Attachment{
		Filename: "study.txt", Type: "text/plain", Content: []byte("知识卡片需要保留来源。\n自己的理解与原文分开。"),
	}})
	require.NoError(t, err)
	document, err := ts.Service.CreateDocument(ownerCtx, &v1pb.CreateDocumentRequest{Attachment: attachment.Name})
	require.NoError(t, err)
	content, err := ts.Service.GetDocumentContent(ownerCtx, &v1pb.GetDocumentContentRequest{Name: document.Name})
	require.NoError(t, err)
	require.NotEmpty(t, content.ContentHash)
	_, err = ts.Service.GetDocumentContent(otherCtx, &v1pb.GetDocumentContentRequest{Name: document.Name})
	require.Equal(t, codes.NotFound, status.Code(err))

	topic, err := ts.Service.CreateKnowledgeTopic(ownerCtx, &v1pb.CreateKnowledgeTopicRequest{Topic: &v1pb.KnowledgeTopic{DisplayName: "学习"}})
	require.NoError(t, err)
	_, err = ts.Service.CreateKnowledgeCard(otherCtx, &v1pb.CreateKnowledgeCardRequest{
		Card:   &v1pb.KnowledgeCard{Title: "越权", Body: "不允许", Topics: []string{topic.Name}},
		Source: &v1pb.CardSourceSelection{Document: document.Name, ContentHash: content.ContentHash, StartOffset: 0, EndOffset: 4},
	})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = ts.Service.CreateKnowledgeCard(ownerCtx, &v1pb.CreateKnowledgeCardRequest{
		Card:   &v1pb.KnowledgeCard{Title: "过时", Body: "不允许"},
		Source: &v1pb.CardSourceSelection{Document: document.Name, ContentHash: "stale", StartOffset: 0, EndOffset: 4},
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	card, err := ts.Service.CreateKnowledgeCard(ownerCtx, &v1pb.CreateKnowledgeCardRequest{
		Card:   &v1pb.KnowledgeCard{Title: "保留来源", Body: "我的理解：记录想法，也能回看原文。", Topics: []string{topic.Name}},
		Source: &v1pb.CardSourceSelection{Document: document.Name, ContentHash: content.ContentHash, StartOffset: 0, EndOffset: 4},
	})
	require.NoError(t, err)
	require.Equal(t, "知识卡片", card.SourceQuote)
	require.Equal(t, "text:0:4", card.SourceLocator)
	require.True(t, card.SourceAvailable)
	require.Equal(t, []string{topic.Name}, card.Topics)
	plan, err := ts.Service.GetDocumentDeletePlan(ownerCtx, &v1pb.GetDocumentDeletePlanRequest{Name: document.Name})
	require.NoError(t, err)
	require.Equal(t, int32(1), plan.LinkedCardCount)
	otherList, err := ts.Service.ListKnowledgeCards(otherCtx, &v1pb.ListKnowledgeCardsRequest{})
	require.NoError(t, err)
	require.Empty(t, otherList.Cards)
	_, err = ts.Service.UpdateKnowledgeCard(otherCtx, &v1pb.UpdateKnowledgeCardRequest{Card: &v1pb.KnowledgeCard{
		Name: card.Name, Title: "越权", Body: "不允许",
	}})
	require.Equal(t, codes.NotFound, status.Code(err))

	updated, err := ts.Service.UpdateKnowledgeCard(ownerCtx, &v1pb.UpdateKnowledgeCardRequest{Card: &v1pb.KnowledgeCard{
		Name: card.Name, Title: "新标题", Body: "自己的新理解", CardType: "idea", Topics: []string{topic.Name}, SourceQuote: "篡改",
	}})
	require.NoError(t, err)
	require.Equal(t, "知识卡片", updated.SourceQuote)
	search, err := ts.Service.SearchKnowledge(ownerCtx, &v1pb.SearchKnowledgeRequest{Query: "新标题", Topic: topic.Name})
	require.NoError(t, err)
	require.Len(t, search.Results, 1)
	require.Equal(t, card.Name, search.Results[0].Card)

	updated.Archived = true
	_, err = ts.Service.UpdateKnowledgeCard(ownerCtx, &v1pb.UpdateKnowledgeCardRequest{Card: updated})
	require.NoError(t, err)
	search, err = ts.Service.SearchKnowledge(ownerCtx, &v1pb.SearchKnowledgeRequest{Query: "新标题"})
	require.NoError(t, err)
	require.Empty(t, search.Results)

	_, err = ts.Service.DeleteDocument(ownerCtx, &v1pb.DeleteDocumentRequest{Name: document.Name, Confirm: true})
	require.NoError(t, err)
	list, err := ts.Service.ListKnowledgeCards(ownerCtx, &v1pb.ListKnowledgeCardsRequest{IncludeArchived: true})
	require.NoError(t, err)
	require.Len(t, list.Cards, 1)
	require.False(t, list.Cards[0].SourceAvailable)
	require.Equal(t, "知识卡片", list.Cards[0].SourceQuote)
}
