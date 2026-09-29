import { create } from "@bufbuild/protobuf";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { documentServiceClient } from "@/connect";
import {
  CreateDocumentRequestSchema,
  DeleteDocumentRequestSchema,
  GetDocumentContentRequestSchema,
  GetDocumentDeletePlanRequestSchema,
  ListDocumentsRequestSchema,
  RetryDocumentRequestSchema,
} from "@/types/proto/api/v1/document_service_pb";

export const documentKeys = {
  all: ["documents"] as const,
  list: () => [...documentKeys.all, "list"] as const,
  content: (name: string) => [...documentKeys.all, "content", name] as const,
};

export function useDocumentContent(name: string) {
  return useQuery({
    queryKey: documentKeys.content(name),
    queryFn: () => documentServiceClient.getDocumentContent(create(GetDocumentContentRequestSchema, { name })),
    enabled: Boolean(name),
  });
}

export function useDocuments() {
  return useQuery({
    queryKey: documentKeys.list(),
    queryFn: async () => {
      const response = await documentServiceClient.listDocuments(create(ListDocumentsRequestSchema, { pageSize: 100 }));
      return response.documents;
    },
  });
}

export function useCreateDocument() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (attachment: string) => documentServiceClient.createDocument(create(CreateDocumentRequestSchema, { attachment })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: documentKeys.list() }),
  });
}

export function useRetryDocument() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (name: string) => documentServiceClient.retryDocument(create(RetryDocumentRequestSchema, { name })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: documentKeys.list() }),
  });
}

export function useDeleteDocument() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (name: string) => {
      await documentServiceClient.deleteDocument(create(DeleteDocumentRequestSchema, { name, confirm: true }));
      return name;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: documentKeys.list() }),
  });
}

export async function getDocumentDeletePlan(name: string) {
  return documentServiceClient.getDocumentDeletePlan(create(GetDocumentDeletePlanRequestSchema, { name }));
}
