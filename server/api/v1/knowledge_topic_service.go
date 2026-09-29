package v1

import (
	"context"
	"strings"
	"time"

	"github.com/lithammer/shortuuid/v4"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) CreateKnowledgeTopic(ctx context.Context, req *v1pb.CreateKnowledgeTopicRequest) (*v1pb.KnowledgeTopic, error) {
	u, e := s.requireDocumentUser(ctx)
	if e != nil {
		return nil, e
	}
	if req.GetTopic() == nil {
		return nil, status.Error(codes.InvalidArgument, "topic is required")
	}
	parent, e := s.topicParentID(ctx, req.Topic.GetParent(), u.ID)
	if e != nil {
		return nil, e
	}
	t, e := s.Store.CreateKnowledgeTopic(ctx, &store.KnowledgeTopic{UID: shortuuid.New(), CreatorID: u.ID, ParentID: parent, Name: req.Topic.GetDisplayName(), Description: req.Topic.GetDescription()})
	if e != nil {
		return nil, topicStoreError(e)
	}
	return convertKnowledgeTopic(t), nil
}
func (s *APIV1Service) ListKnowledgeTopics(ctx context.Context, _ *v1pb.ListKnowledgeTopicsRequest) (*v1pb.ListKnowledgeTopicsResponse, error) {
	u, e := s.requireDocumentUser(ctx)
	if e != nil {
		return nil, e
	}
	items, e := s.Store.ListKnowledgeTopics(ctx, u.ID)
	if e != nil {
		return nil, status.Error(codes.Internal, "failed to list topics")
	}
	out := &v1pb.ListKnowledgeTopicsResponse{}
	for _, t := range items {
		out.Topics = append(out.Topics, convertKnowledgeTopic(t))
	}
	return out, nil
}
func (s *APIV1Service) UpdateKnowledgeTopic(ctx context.Context, req *v1pb.UpdateKnowledgeTopicRequest) (*v1pb.KnowledgeTopic, error) {
	u, e := s.requireDocumentUser(ctx)
	if e != nil {
		return nil, e
	}
	if req.GetTopic() == nil {
		return nil, status.Error(codes.InvalidArgument, "topic is required")
	}
	t, e := s.getOwnedTopic(ctx, req.Topic.GetName(), u.ID)
	if e != nil {
		return nil, e
	}
	parent, e := s.topicParentID(ctx, req.Topic.GetParent(), u.ID)
	if e != nil {
		return nil, e
	}
	t.ParentID = parent
	t.Name = req.Topic.GetDisplayName()
	t.Description = req.Topic.GetDescription()
	t, e = s.Store.UpdateKnowledgeTopic(ctx, t)
	if e != nil {
		return nil, topicStoreError(e)
	}
	return convertKnowledgeTopic(t), nil
}
func (s *APIV1Service) DeleteKnowledgeTopic(ctx context.Context, req *v1pb.DeleteKnowledgeTopicRequest) (*emptypb.Empty, error) {
	u, e := s.requireDocumentUser(ctx)
	if e != nil {
		return nil, e
	}
	t, e := s.getOwnedTopic(ctx, req.GetName(), u.ID)
	if e != nil {
		return nil, e
	}
	if e = s.Store.DeleteKnowledgeTopic(ctx, t.ID, u.ID); e != nil {
		return nil, topicStoreError(e)
	}
	return &emptypb.Empty{}, nil
}
func (s *APIV1Service) SetDocumentTopics(ctx context.Context, req *v1pb.SetDocumentTopicsRequest) (*emptypb.Empty, error) {
	u, e := s.requireDocumentUser(ctx)
	if e != nil {
		return nil, e
	}
	d, e := s.getOwnedDocument(ctx, req.GetDocument(), u.ID)
	if e != nil {
		return nil, e
	}
	ids := make([]int32, 0, len(req.Topics))
	seen := map[int32]bool{}
	for _, name := range req.Topics {
		t, e := s.getOwnedTopic(ctx, name, u.ID)
		if e != nil {
			return nil, e
		}
		if !seen[t.ID] {
			seen[t.ID] = true
			ids = append(ids, t.ID)
		}
	}
	if e = s.Store.SetDocumentTopics(ctx, d.ID, u.ID, ids); e != nil {
		return nil, status.Error(codes.Internal, "failed to update document topics")
	}
	return &emptypb.Empty{}, nil
}
func (s *APIV1Service) ListDocumentTopics(ctx context.Context, req *v1pb.ListDocumentTopicsRequest) (*v1pb.ListKnowledgeTopicsResponse, error) {
	u, e := s.requireDocumentUser(ctx)
	if e != nil {
		return nil, e
	}
	d, e := s.getOwnedDocument(ctx, req.GetDocument(), u.ID)
	if e != nil {
		return nil, e
	}
	ids, e := s.Store.ListDocumentTopicIDs(ctx, d.ID, u.ID)
	if e != nil {
		return nil, status.Error(codes.Internal, "failed to list document topics")
	}
	out := &v1pb.ListKnowledgeTopicsResponse{}
	for _, id := range ids {
		t, e := s.Store.GetKnowledgeTopic(ctx, &store.FindKnowledgeTopic{ID: &id, CreatorID: u.ID})
		if e != nil {
			return nil, status.Error(codes.Internal, "failed to read topic")
		}
		if t != nil {
			out.Topics = append(out.Topics, convertKnowledgeTopic(t))
		}
	}
	return out, nil
}

func ExtractKnowledgeTopicUIDFromName(name string) (string, error) {
	const p = "knowledge/topics/"
	if !strings.HasPrefix(name, p) || strings.TrimPrefix(name, p) == "" || strings.Contains(strings.TrimPrefix(name, p), "/") {
		return "", errors.New("invalid knowledge topic name")
	}
	return strings.TrimPrefix(name, p), nil
}
func (s *APIV1Service) getOwnedTopic(ctx context.Context, name string, creatorID int32) (*store.KnowledgeTopic, error) {
	uid, e := ExtractKnowledgeTopicUIDFromName(name)
	if e != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid topic name")
	}
	t, e := s.Store.GetKnowledgeTopic(ctx, &store.FindKnowledgeTopic{UID: &uid, CreatorID: creatorID})
	if e != nil {
		return nil, status.Error(codes.Internal, "failed to read topic")
	}
	if t == nil {
		return nil, status.Error(codes.NotFound, "topic not found")
	}
	return t, nil
}
func (s *APIV1Service) topicParentID(ctx context.Context, name string, creatorID int32) (*int32, error) {
	if name == "" {
		return nil, nil
	}
	t, e := s.getOwnedTopic(ctx, name, creatorID)
	if e != nil {
		return nil, e
	}
	return &t.ID, nil
}
func convertKnowledgeTopic(t *store.KnowledgeTopic) *v1pb.KnowledgeTopic {
	out := &v1pb.KnowledgeTopic{Name: "knowledge/topics/" + t.UID, DisplayName: t.Name, Description: t.Description, CreateTime: timestamppb.New(time.Unix(t.CreatedTs, 0)), UpdateTime: timestamppb.New(time.Unix(t.UpdatedTs, 0))}
	if t.ParentUID != "" {
		out.Parent = "knowledge/topics/" + t.ParentUID
	}
	return out
}
func topicStoreError(e error) error {
	m := e.Error()
	switch {
	case strings.Contains(m, "cycle"):
		return status.Error(codes.InvalidArgument, "topic hierarchy cannot contain a cycle")
	case strings.Contains(m, "children"):
		return status.Error(codes.FailedPrecondition, "move or delete child topics first")
	case strings.Contains(m, "UNIQUE"), strings.Contains(m, "Duplicate"):
		return status.Error(codes.AlreadyExists, "a topic with this name already exists here")
	default:
		return status.Error(codes.InvalidArgument, "topic could not be saved")
	}
}
