import { create } from "@bufbuild/protobuf";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { knowledgeTopicServiceClient } from "@/connect";
import {
  CreateKnowledgeTopicRequestSchema,
  DeleteKnowledgeTopicRequestSchema,
  KnowledgeTopicSchema,
  ListDocumentTopicsRequestSchema,
  ListKnowledgeTopicsRequestSchema,
  SetDocumentTopicsRequestSchema,
  UpdateKnowledgeTopicRequestSchema,
} from "@/types/proto/api/v1/knowledge_topic_service_pb";

export const topicKeys = {
  all: ["knowledge-topics"] as const,
  document: (name: string) => ["knowledge-topics", "document", name] as const,
};
export const useKnowledgeTopics = () =>
  useQuery({
    queryKey: topicKeys.all,
    queryFn: async () => (await knowledgeTopicServiceClient.listKnowledgeTopics(create(ListKnowledgeTopicsRequestSchema))).topics,
  });
export const useDocumentTopics = (document: string) =>
  useQuery({
    queryKey: topicKeys.document(document),
    queryFn: async () =>
      (await knowledgeTopicServiceClient.listDocumentTopics(create(ListDocumentTopicsRequestSchema, { document }))).topics,
    enabled: Boolean(document),
  });
export function useSaveKnowledgeTopic() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { name?: string; displayName: string; description: string; parent: string }) =>
      input.name
        ? knowledgeTopicServiceClient.updateKnowledgeTopic(
            create(UpdateKnowledgeTopicRequestSchema, {
              topic: create(KnowledgeTopicSchema, {
                name: input.name,
                displayName: input.displayName,
                description: input.description,
                parent: input.parent,
              }),
            }),
          )
        : knowledgeTopicServiceClient.createKnowledgeTopic(
            create(CreateKnowledgeTopicRequestSchema, {
              topic: create(KnowledgeTopicSchema, { displayName: input.displayName, description: input.description, parent: input.parent }),
            }),
          ),
    onSuccess: () => qc.invalidateQueries({ queryKey: topicKeys.all }),
  });
}
export function useDeleteKnowledgeTopic() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => knowledgeTopicServiceClient.deleteKnowledgeTopic(create(DeleteKnowledgeTopicRequestSchema, { name })),
    onSuccess: () => qc.invalidateQueries({ queryKey: topicKeys.all }),
  });
}
export function useSetDocumentTopics() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ document, topics }: { document: string; topics: string[] }) =>
      knowledgeTopicServiceClient.setDocumentTopics(create(SetDocumentTopicsRequestSchema, { document, topics })),
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: topicKeys.document(v.document) }),
  });
}
