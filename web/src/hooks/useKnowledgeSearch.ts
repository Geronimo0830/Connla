import { create } from "@bufbuild/protobuf";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { knowledgeSearchServiceClient } from "@/connect";
import { RebuildKnowledgeSearchRequestSchema, SearchKnowledgeRequestSchema } from "@/types/proto/api/v1/knowledge_search_service_pb";

export const toSearchDateBound = (value: string, endExclusive: boolean): bigint => {
  if (!value) return 0n;
  const date = new Date(`${value}T00:00:00`);
  if (Number.isNaN(date.getTime())) return 0n;
  if (endExclusive) date.setDate(date.getDate() + 1);
  return BigInt(Math.floor(date.getTime() / 1000));
};

export const searchKeys = {
  all: ["knowledge-search"] as const,
  query: (query: string, topic: string, contentType: string, fileFormat: string, createdFrom: bigint, createdBefore: bigint) =>
    ["knowledge-search", query, topic, contentType, fileFormat, createdFrom.toString(), createdBefore.toString()] as const,
};
export const useKnowledgeSearch = (
  query: string,
  topic: string,
  contentType: string,
  fileFormat: string,
  createdFrom: bigint,
  createdBefore: bigint,
  validRange: boolean,
) =>
  useQuery({
    queryKey: searchKeys.query(query, topic, contentType, fileFormat, createdFrom, createdBefore),
    queryFn: async () =>
      (
        await knowledgeSearchServiceClient.searchKnowledge(
          create(SearchKnowledgeRequestSchema, { query, topic, contentType, fileFormat, createdFrom, createdBefore, pageSize: 50 }),
        )
      ).results,
    enabled: query.trim().length > 0 && validRange,
  });
export const useRebuildKnowledgeSearch = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => knowledgeSearchServiceClient.rebuildKnowledgeSearch(create(RebuildKnowledgeSearchRequestSchema)),
    onSuccess: () => qc.invalidateQueries({ queryKey: searchKeys.all }),
  });
};
