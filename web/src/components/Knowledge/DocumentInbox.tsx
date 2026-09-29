import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
  AlertCircleIcon,
  CheckCircle2Icon,
  DownloadIcon,
  EyeIcon,
  FileTextIcon,
  LoaderCircleIcon,
  RefreshCwIcon,
  SearchIcon,
  Trash2Icon,
  UploadIcon,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { useSearchParams } from "react-router-dom";
import ConfirmDialog from "@/components/ConfirmDialog";
import DocumentTextReader from "@/components/Knowledge/DocumentTextReader";
import PdfReaderDialog from "@/components/Knowledge/PdfReaderDialog";
import { uploadService } from "@/components/MemoEditor/services/uploadService";
import { Badge, type BadgeVariant } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { getDocumentDeletePlan, useCreateDocument, useDeleteDocument, useDocuments, useRetryDocument } from "@/hooks/useDocumentQueries";
import { cn } from "@/lib/utils";
import { type Document, type DocumentDeletePlan, DocumentStatus } from "@/types/proto/api/v1/document_service_pb";
import { formatFileSize } from "@/utils/format";
import { useTranslate } from "@/utils/i18n";

type Filter = "all" | "ready" | "attention";
type UploadState = "uploading" | "registering" | "complete" | "failed";

const FILTER_LABELS = {
  all: "knowledge.documents.filter-all",
  ready: "knowledge.documents.filter-ready",
  attention: "knowledge.documents.filter-attention",
} as const;

const STATUS_LABELS = {
  ready: "knowledge.documents.status-ready",
  working: "knowledge.documents.status-working",
  failed: "knowledge.documents.status-failed",
  unsupported: "knowledge.documents.status-unsupported",
} as const;

const DOCUMENT_MEDIA_TYPES: Record<string, string> = {
  txt: "text/plain",
  md: "text/markdown",
  markdown: "text/markdown",
  pdf: "application/pdf",
  doc: "application/msword",
  docx: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  xls: "application/vnd.ms-excel",
  xlsx: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
};

interface UploadItem {
  id: string;
  filename: string;
  progress: number;
  state: UploadState;
  error?: string;
}

const STATUS_PRESENTATION: Record<
  DocumentStatus,
  { label: "ready" | "working" | "failed" | "unsupported"; variant: BadgeVariant; icon: typeof CheckCircle2Icon }
> = {
  [DocumentStatus.UNSPECIFIED]: { label: "working", variant: "outline", icon: LoaderCircleIcon },
  [DocumentStatus.UPLOADED]: { label: "working", variant: "outline", icon: LoaderCircleIcon },
  [DocumentStatus.QUEUED]: { label: "working", variant: "outline", icon: LoaderCircleIcon },
  [DocumentStatus.PARSING]: { label: "working", variant: "outline", icon: LoaderCircleIcon },
  [DocumentStatus.READY]: { label: "ready", variant: "secondary", icon: CheckCircle2Icon },
  [DocumentStatus.FAILED]: { label: "failed", variant: "destructive", icon: AlertCircleIcon },
  [DocumentStatus.UNSUPPORTED]: { label: "unsupported", variant: "warning", icon: AlertCircleIcon },
};

const documentDownloadUrl = (document: Document) =>
  `${window.location.origin}/file/${document.sourceAttachment}/${encodeURIComponent(document.originalFilename)}`;

const normalizeDocumentFile = (file: File): File => {
  const extension = file.name.split(".").pop()?.toLocaleLowerCase() ?? "";
  const type = DOCUMENT_MEDIA_TYPES[extension];
  return type && file.type !== type ? new File([file], file.name, { type, lastModified: file.lastModified }) : file;
};

const DocumentInbox = () => {
  const t = useTranslate();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { data: documents = [], isLoading, isError, refetch } = useDocuments();
  const createDocument = useCreateDocument();
  const retryDocument = useRetryDocument();
  const deleteDocument = useDeleteDocument();
  const [uploads, setUploads] = useState<UploadItem[]>([]);
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const [deleteTarget, setDeleteTarget] = useState<{ document: Document; plan: DocumentDeletePlan }>();
  const [readerTarget, setReaderTarget] = useState<Document>();
  const [textTarget, setTextTarget] = useState<Document>();
  const [searchParams, setSearchParams] = useSearchParams();

  useEffect(() => {
    const name = searchParams.get("document");
    if (!name || !documents.length) return;
    const found = documents.find((item) => item.name === name);
    if (found?.status === DocumentStatus.READY) setTextTarget(found);
  }, [documents, searchParams]);

  const filteredDocuments = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase();
    return documents.filter((document) => {
      const matchesQuery =
        !normalizedQuery || `${document.title} ${document.originalFilename}`.toLocaleLowerCase().includes(normalizedQuery);
      if (!matchesQuery) return false;
      if (filter === "ready") return document.status === DocumentStatus.READY;
      if (filter === "attention") return document.status === DocumentStatus.FAILED || document.status === DocumentStatus.UNSUPPORTED;
      return true;
    });
  }, [documents, filter, query]);

  const updateUpload = (id: string, patch: Partial<UploadItem>) => {
    setUploads((current) => current.map((item) => (item.id === id ? { ...item, ...patch } : item)));
  };

  const uploadOne = async (file: File, id: string) => {
    try {
      const normalizedFile = normalizeDocumentFile(file);
      const attachment = await uploadService.uploadFile({ file: normalizedFile, previewUrl: "", origin: "upload" }, undefined, (progress) =>
        updateUpload(id, { progress }),
      );
      updateUpload(id, { state: "registering", progress: 100 });
      await createDocument.mutateAsync(attachment.name);
      updateUpload(id, { state: "complete" });
    } catch (error) {
      console.error("Document upload failed", error);
      updateUpload(id, { state: "failed", error: t("knowledge.documents.upload-error") });
    }
  };

  const handleFiles = (files: FileList | null) => {
    if (!files?.length) return;
    const pending = Array.from(files).map((file) => ({
      file,
      item: { id: crypto.randomUUID(), filename: file.name, progress: 0, state: "uploading" as const },
    }));
    setUploads((current) => [...pending.map(({ item }) => item), ...current]);
    void Promise.all(pending.map(({ file, item }) => uploadOne(file, item.id)));
    if (fileInputRef.current) fileInputRef.current.value = "";
  };

  const handleRetry = async (document: Document) => {
    try {
      await retryDocument.mutateAsync(document.name);
      toast.success(t("knowledge.documents.retry-success"));
    } catch {
      toast.error(t("knowledge.documents.retry-error"));
    }
  };

  const prepareDelete = async (document: Document) => {
    try {
      const plan = await getDocumentDeletePlan(document.name);
      setDeleteTarget({ document, plan });
    } catch {
      toast.error(t("knowledge.documents.delete-plan-error"));
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    await deleteDocument.mutateAsync(deleteTarget.document.name);
    toast.success(t("knowledge.documents.delete-success"));
    setDeleteTarget(undefined);
  };

  return (
    <div className="mt-8 space-y-6">
      <section className="rounded-xl border border-border bg-muted/20 p-4 sm:p-5" aria-labelledby="document-upload-title">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 id="document-upload-title" className="text-base font-medium text-foreground">
              {t("knowledge.documents.upload-title")}
            </h2>
          </div>
          <Button className="h-11 shrink-0 px-4" onClick={() => fileInputRef.current?.click()}>
            <UploadIcon aria-hidden="true" />
            {t("knowledge.documents.choose-files")}
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            multiple
            className="sr-only"
            accept=".txt,.md,.markdown,.pdf,.doc,.docx,.xls,.xlsx"
            aria-label={t("knowledge.documents.choose-files")}
            onChange={(event) => handleFiles(event.currentTarget.files)}
          />
        </div>

        {uploads.length > 0 && (
          <div className="mt-4 space-y-2" aria-live="polite" aria-label={t("knowledge.documents.upload-progress")}>
            {uploads.map((upload) => (
              <div key={upload.id} className="rounded-lg border border-border bg-background px-3 py-2.5">
                <div className="flex items-center gap-3">
                  <FileTextIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                  <span className="min-w-0 flex-1 truncate text-sm text-foreground">{upload.filename}</span>
                  <span className="text-xs text-muted-foreground">
                    {upload.state === "complete"
                      ? t("knowledge.documents.upload-complete")
                      : upload.state === "failed"
                        ? t("knowledge.documents.upload-failed")
                        : upload.state === "registering"
                          ? t("knowledge.documents.processing")
                          : `${upload.progress}%`}
                  </span>
                </div>
                {upload.state === "uploading" && (
                  <div
                    className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted"
                    role="progressbar"
                    aria-valuenow={upload.progress}
                    aria-valuemin={0}
                    aria-valuemax={100}
                  >
                    <div
                      className="h-full rounded-full bg-primary transition-[width] duration-200 motion-reduce:transition-none"
                      style={{ width: `${upload.progress}%` }}
                    />
                  </div>
                )}
                {upload.state === "failed" && upload.error && (
                  <p className="mt-1 text-xs text-destructive" role="alert">
                    {upload.error}
                  </p>
                )}
              </div>
            ))}
          </div>
        )}
      </section>

      <section aria-labelledby="document-list-title">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h2 id="document-list-title" className="text-lg font-semibold text-foreground">
              {t("knowledge.documents.list-title")}
            </h2>
            <p className="mt-1 text-sm text-muted-foreground">{t("knowledge.documents.list-count", { count: documents.length })}</p>
          </div>
          <label className="relative block w-full sm:max-w-64">
            <span className="sr-only">{t("knowledge.documents.search")}</span>
            <SearchIcon
              className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
              aria-hidden="true"
            />
            <Input
              value={query}
              onChange={(event) => setQuery(event.currentTarget.value)}
              className="h-11 pl-9"
              placeholder={t("knowledge.documents.search")}
            />
          </label>
        </div>

        <div className="mt-4 flex flex-wrap gap-2" aria-label={t("knowledge.documents.filters")}>
          {(["all", "ready", "attention"] as const).map((value) => (
            <Button
              key={value}
              variant={filter === value ? "secondary" : "outline"}
              className="h-11"
              aria-pressed={filter === value}
              onClick={() => setFilter(value)}
            >
              {t(FILTER_LABELS[value])}
            </Button>
          ))}
        </div>

        {isLoading ? (
          <div className="mt-4 rounded-lg border border-border p-8 text-center text-sm text-muted-foreground" role="status">
            {t("knowledge.documents.loading")}
          </div>
        ) : isError ? (
          <div className="mt-4 rounded-lg border border-destructive/30 bg-destructive/5 p-5" role="alert">
            <p className="text-sm text-foreground">{t("knowledge.documents.load-error")}</p>
            <Button variant="outline" className="mt-3 h-11" onClick={() => refetch()}>
              {t("knowledge.documents.try-again")}
            </Button>
          </div>
        ) : filteredDocuments.length === 0 ? (
          <div className="mt-4 rounded-lg border border-dashed border-border p-8 text-center">
            <p className="text-sm font-medium text-foreground">
              {documents.length ? t("knowledge.documents.no-results") : t("knowledge.documents.empty")}
            </p>
          </div>
        ) : (
          <ul className="mt-4 divide-y divide-border rounded-lg border border-border bg-background">
            {filteredDocuments.map((document) => {
              const status = STATUS_PRESENTATION[document.status] ?? STATUS_PRESENTATION[DocumentStatus.UNSPECIFIED];
              const StatusIcon = status.icon;
              return (
                <li key={document.name} className="p-4 sm:p-5">
                  <div className="flex min-w-0 flex-col gap-4 sm:flex-row sm:items-start">
                    <div className="flex min-w-0 flex-1 items-start gap-3">
                      <span
                        className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground"
                        aria-hidden="true"
                      >
                        <FileTextIcon className="size-5" strokeWidth={1.8} />
                      </span>
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="break-words text-sm font-medium text-foreground">{document.title || document.originalFilename}</h3>
                          <Badge variant={status.variant}>
                            <StatusIcon
                              className={cn(status.label === "working" && "animate-spin motion-reduce:animate-none")}
                              aria-hidden="true"
                            />
                            {t(STATUS_LABELS[status.label])}
                          </Badge>
                        </div>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {document.originalFilename} · {formatFileSize(Number(document.size))}
                          {document.createTime ? ` · ${timestampDate(document.createTime).toLocaleDateString()}` : ""}
                        </p>
                        {(document.status === DocumentStatus.FAILED || document.status === DocumentStatus.UNSUPPORTED) && (
                          <p
                            className={cn(
                              "mt-2 text-sm leading-6",
                              document.status === DocumentStatus.FAILED ? "text-destructive" : "text-warning",
                            )}
                            role="status"
                          >
                            {document.status === DocumentStatus.UNSUPPORTED &&
                            (document.extension === ".doc" || document.extension === ".xls")
                              ? "这是旧版 Office 文件，暂不支持内容识别。请转换为 .docx 或 .xlsx 后重新导入；原文件仍可下载。"
                              : document.errorMessage || t("knowledge.documents.unknown-error")}
                          </p>
                        )}
                      </div>
                    </div>
                    <div className="flex flex-wrap gap-2 sm:justify-end">
                      {document.status === DocumentStatus.READY && (
                        <Button variant="default" className="h-11" onClick={() => setTextTarget(document)}>
                          <EyeIcon aria-hidden="true" />
                          阅读文本 / 摘录
                        </Button>
                      )}
                      {document.status === DocumentStatus.READY && document.extension === ".pdf" && (
                        <Button variant="outline" className="h-11" onClick={() => setReaderTarget(document)}>
                          <EyeIcon aria-hidden="true" />
                          {t("knowledge.documents.read")}
                        </Button>
                      )}
                      <a href={documentDownloadUrl(document)} download className={cn(buttonVariants({ variant: "outline" }), "h-11")}>
                        <DownloadIcon aria-hidden="true" />
                        {t("knowledge.documents.download")}
                      </a>
                      {document.status === DocumentStatus.FAILED && (
                        <Button variant="outline" className="h-11" disabled={retryDocument.isPending} onClick={() => handleRetry(document)}>
                          <RefreshCwIcon aria-hidden="true" />
                          {t("knowledge.documents.retry")}
                        </Button>
                      )}
                      <Button
                        variant="ghost"
                        className="h-11 text-destructive hover:text-destructive"
                        onClick={() => prepareDelete(document)}
                      >
                        <Trash2Icon aria-hidden="true" />
                        {t("knowledge.documents.delete")}
                      </Button>
                    </div>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </section>

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => !open && setDeleteTarget(undefined)}
        title={t("knowledge.documents.delete-title", { name: deleteTarget?.document.originalFilename ?? "" })}
        description={
          deleteTarget
            ? `${t("knowledge.documents.delete-description", {
                content: deleteTarget.plan.derivedContentRows,
                attempts: deleteTarget.plan.parseAttemptRows,
              })} 关联的 ${deleteTarget.plan.linkedCardCount} 张卡片会保留原文摘录，但无法再回跳到此文档。`
            : undefined
        }
        confirmLabel={t("knowledge.documents.delete-confirm")}
        cancelLabel={t("common.cancel")}
        confirmVariant="destructive"
        onConfirm={confirmDelete}
      />
      {readerTarget && (
        <PdfReaderDialog
          open
          onOpenChange={(open) => !open && setReaderTarget(undefined)}
          title={readerTarget.title || readerTarget.originalFilename}
          url={documentDownloadUrl(readerTarget)}
        />
      )}
      {textTarget && (
        <DocumentTextReader
          document={textTarget}
          locator={searchParams.get("locator") ?? undefined}
          onClose={() => {
            setTextTarget(undefined);
            if (searchParams.has("document")) setSearchParams({});
          }}
        />
      )}
    </div>
  );
};

export default DocumentInbox;
