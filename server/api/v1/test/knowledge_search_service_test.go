package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

func TestKnowledgeSearchChineseOwnershipTopicFilterAndRebuild(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()
	ctx := context.Background()
	owner, err := ts.CreateRegularUser(ctx, "search-owner")
	require.NoError(t, err)
	other, err := ts.CreateRegularUser(ctx, "search-other")
	require.NoError(t, err)
	ownerCtx := ts.CreateUserContext(ctx, owner.ID)
	otherCtx := ts.CreateUserContext(ctx, other.ID)
	attachment, err := ts.Service.CreateAttachment(ownerCtx, &v1pb.CreateAttachmentRequest{Attachment: &v1pb.Attachment{Filename: "notes.md", Type: "text/markdown", Content: []byte("# 学习\n个人知识管理需要持续整理和检索。")}})
	require.NoError(t, err)
	doc, err := ts.Service.CreateDocument(ownerCtx, &v1pb.CreateDocumentRequest{Attachment: attachment.Name, Title: "知识系统"})
	require.NoError(t, err)
	topic, err := ts.Service.CreateKnowledgeTopic(ownerCtx, &v1pb.CreateKnowledgeTopicRequest{Topic: &v1pb.KnowledgeTopic{DisplayName: "学习方法"}})
	require.NoError(t, err)
	_, err = ts.Service.SetDocumentTopics(ownerCtx, &v1pb.SetDocumentTopicsRequest{Document: doc.Name, Topics: []string{topic.Name}})
	require.NoError(t, err)

	result, err := ts.Service.SearchKnowledge(ownerCtx, &v1pb.SearchKnowledgeRequest{Query: "知识管理"})
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.Equal(t, doc.Name, result.Results[0].Document)
	require.Contains(t, result.Results[0].Snippet, "<mark>")
	card, err := ts.Service.CreateKnowledgeCard(ownerCtx, &v1pb.CreateKnowledgeCardRequest{
		Card: &v1pb.KnowledgeCard{Title: "知识管理卡片", Body: "知识管理需要回看资料。"},
	})
	require.NoError(t, err)
	both, err := ts.Service.SearchKnowledge(ownerCtx, &v1pb.SearchKnowledgeRequest{Query: "知识管理"})
	require.NoError(t, err)
	require.Len(t, both.Results, 2)
	documents, err := ts.Service.SearchKnowledge(ownerCtx, &v1pb.SearchKnowledgeRequest{Query: "知识管理", ContentType: "document", FileFormat: ".md"})
	require.NoError(t, err)
	require.Len(t, documents.Results, 1)
	require.Equal(t, doc.Name, documents.Results[0].Document)
	cards, err := ts.Service.SearchKnowledge(ownerCtx, &v1pb.SearchKnowledgeRequest{Query: "知识管理", ContentType: "card"})
	require.NoError(t, err)
	require.Len(t, cards.Results, 1)
	require.Equal(t, card.Name, cards.Results[0].Card)
	wrongFormat, err := ts.Service.SearchKnowledge(ownerCtx, &v1pb.SearchKnowledgeRequest{Query: "知识管理", FileFormat: ".pdf"})
	require.NoError(t, err)
	require.Empty(t, wrongFormat.Results)
	nowSec := time.Now().Unix()
	withinDate, err := ts.Service.SearchKnowledge(ownerCtx, &v1pb.SearchKnowledgeRequest{Query: "知识管理", CreatedFrom: nowSec - 60, CreatedBefore: nowSec + 60})
	require.NoError(t, err)
	require.Len(t, withinDate.Results, 2)
	futureDate, err := ts.Service.SearchKnowledge(ownerCtx, &v1pb.SearchKnowledgeRequest{Query: "知识管理", CreatedFrom: nowSec + 86400})
	require.NoError(t, err)
	require.Empty(t, futureDate.Results)
	for _, invalid := range []*v1pb.SearchKnowledgeRequest{
		{Query: "知识管理", ContentType: "other"},
		{Query: "知识管理", FileFormat: ".exe"},
		{Query: "知识管理", ContentType: "card", FileFormat: ".md"},
		{Query: "知识管理", CreatedFrom: nowSec + 10, CreatedBefore: nowSec},
	} {
		_, err = ts.Service.SearchKnowledge(ownerCtx, invalid)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	filtered, err := ts.Service.SearchKnowledge(ownerCtx, &v1pb.SearchKnowledgeRequest{Query: "知识管理", Topic: topic.Name})
	require.NoError(t, err)
	require.Len(t, filtered.Results, 1)
	private, err := ts.Service.SearchKnowledge(otherCtx, &v1pb.SearchKnowledgeRequest{Query: "知识管理"})
	require.NoError(t, err)
	require.Empty(t, private.Results)

	_, err = ts.Store.GetDriver().GetDB().ExecContext(ctx, "DELETE FROM knowledge_search")
	require.NoError(t, err)
	empty, err := ts.Service.SearchKnowledge(ownerCtx, &v1pb.SearchKnowledgeRequest{Query: "知识管理"})
	require.NoError(t, err)
	require.Empty(t, empty.Results)
	rebuilt, err := ts.Service.RebuildKnowledgeSearch(ownerCtx, &v1pb.RebuildKnowledgeSearchRequest{})
	require.NoError(t, err)
	require.Equal(t, int32(1), rebuilt.IndexedDocuments)
	require.Equal(t, int32(1), rebuilt.IndexedCards)
	repaired, err := ts.Service.SearchKnowledge(ownerCtx, &v1pb.SearchKnowledgeRequest{Query: "知识管理"})
	require.NoError(t, err)
	require.Len(t, repaired.Results, 2)
}
