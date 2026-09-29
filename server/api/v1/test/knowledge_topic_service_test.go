package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

func TestKnowledgeTopicHierarchyOwnershipAndCycles(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()
	ctx := context.Background()
	owner, err := ts.CreateRegularUser(ctx, "topic-owner")
	require.NoError(t, err)
	other, err := ts.CreateRegularUser(ctx, "topic-other")
	require.NoError(t, err)
	ownerCtx := ts.CreateUserContext(ctx, owner.ID)
	otherCtx := ts.CreateUserContext(ctx, other.ID)

	root, err := ts.Service.CreateKnowledgeTopic(ownerCtx, &v1pb.CreateKnowledgeTopicRequest{Topic: &v1pb.KnowledgeTopic{DisplayName: "Programming"}})
	require.NoError(t, err)
	child, err := ts.Service.CreateKnowledgeTopic(ownerCtx, &v1pb.CreateKnowledgeTopicRequest{Topic: &v1pb.KnowledgeTopic{DisplayName: "Go", Parent: root.Name}})
	require.NoError(t, err)
	require.Equal(t, root.Name, child.Parent)

	listed, err := ts.Service.ListKnowledgeTopics(otherCtx, &v1pb.ListKnowledgeTopicsRequest{})
	require.NoError(t, err)
	require.Empty(t, listed.Topics)
	_, err = ts.Service.UpdateKnowledgeTopic(otherCtx, &v1pb.UpdateKnowledgeTopicRequest{Topic: &v1pb.KnowledgeTopic{Name: root.Name, DisplayName: "Stolen"}})
	require.Equal(t, codes.NotFound, status.Code(err))

	root.Parent = child.Name
	_, err = ts.Service.UpdateKnowledgeTopic(ownerCtx, &v1pb.UpdateKnowledgeTopicRequest{Topic: root})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = ts.Service.DeleteKnowledgeTopic(ownerCtx, &v1pb.DeleteKnowledgeTopicRequest{Name: root.Name})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	_, err = ts.Service.DeleteKnowledgeTopic(ownerCtx, &v1pb.DeleteKnowledgeTopicRequest{Name: child.Name})
	require.NoError(t, err)
	_, err = ts.Service.DeleteKnowledgeTopic(ownerCtx, &v1pb.DeleteKnowledgeTopicRequest{Name: root.Name})
	require.NoError(t, err)
}
