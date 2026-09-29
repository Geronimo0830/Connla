import { create } from "@bufbuild/protobuf";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { knowledgeCardServiceClient } from "@/connect";
import { searchKeys } from "@/hooks/useKnowledgeSearch";
import {
  CardSourceSelectionSchema,
  CreateKnowledgeCardRequestSchema,
  type KnowledgeCard,
  KnowledgeCardSchema,
  ListKnowledgeCardsRequestSchema,
  UpdateKnowledgeCardRequestSchema,
} from "@/types/proto/api/v1/knowledge_card_service_pb";

export interface CardSourceSelection {
  document: string;
  contentHash: string;
  startOffset: number;
  endOffset: number;
  quote: string;
  title: string;
}

export const cardKeys = { all: ["knowledge-cards"] as const };

export const useKnowledgeCards = () =>
  useQuery({
    queryKey: cardKeys.all,
    queryFn: async () =>
      (await knowledgeCardServiceClient.listKnowledgeCards(create(ListKnowledgeCardsRequestSchema, { includeArchived: true }))).cards,
  });

export function useSaveKnowledgeCard() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: {
      card?: KnowledgeCard;
      title: string;
      body: string;
      cardType: string;
      topics: string[];
      archived?: boolean;
      source?: CardSourceSelection;
    }) => {
      const card = create(KnowledgeCardSchema, {
        name: input.card?.name ?? "",
        title: input.title,
        body: input.body,
        cardType: input.cardType,
        topics: input.topics,
        archived: input.archived ?? input.card?.archived ?? false,
      });
      return input.card
        ? knowledgeCardServiceClient.updateKnowledgeCard(create(UpdateKnowledgeCardRequestSchema, { card }))
        : knowledgeCardServiceClient.createKnowledgeCard(
            create(CreateKnowledgeCardRequestSchema, {
              card,
              source: input.source
                ? create(CardSourceSelectionSchema, {
                    document: input.source.document,
                    contentHash: input.source.contentHash,
                    startOffset: input.source.startOffset,
                    endOffset: input.source.endOffset,
                  })
                : undefined,
            }),
          );
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: cardKeys.all });
      await qc.invalidateQueries({ queryKey: searchKeys.all });
    },
  });
}
