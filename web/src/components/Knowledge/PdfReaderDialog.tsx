import { ChevronLeftIcon, ChevronRightIcon, DownloadIcon, LoaderCircleIcon } from "lucide-react";
import type { PDFDocumentProxy, RenderTask } from "pdfjs-dist";
import workerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import { useEffect, useRef, useState } from "react";
import { Button, buttonVariants } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import { useTranslate } from "@/utils/i18n";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  url: string;
}

const PdfReaderDialog = ({ open, onOpenChange, title, url }: Props) => {
  const t = useTranslate();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const documentRef = useRef<PDFDocumentProxy | undefined>(undefined);
  const [pageNumber, setPageNumber] = useState(1);
  const [pageCount, setPageCount] = useState(0);
  const [documentReady, setDocumentReady] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!open) return;
    let disposed = false;
    let loadingTask: ReturnType<typeof import("pdfjs-dist")["getDocument"]> | undefined;
    setLoading(true);
    setError(false);
    setDocumentReady(false);
    setPageNumber(1);

    void import("pdfjs-dist")
      .then(async ({ getDocument, GlobalWorkerOptions }) => {
        GlobalWorkerOptions.workerSrc = workerUrl;
        loadingTask = getDocument({
          url,
          withCredentials: true,
          useSystemFonts: true,
          useWasm: false,
          stopAtErrors: true,
          maxImageSize: 25_000_000,
        });
        const pdf = await loadingTask.promise;
        if (disposed) return;
        documentRef.current = pdf;
        setPageCount(pdf.numPages);
        setDocumentReady(true);
      })
      .catch(() => {
        if (!disposed) {
          setLoading(false);
          setError(true);
        }
      });

    return () => {
      disposed = true;
      void loadingTask?.destroy();
      documentRef.current = undefined;
    };
  }, [open, url]);

  useEffect(() => {
    if (!open || !documentReady || !documentRef.current || !canvasRef.current) return;
    let disposed = false;
    let renderTask: RenderTask | undefined;
    setLoading(true);
    void documentRef.current
      .getPage(pageNumber)
      .then(async (page) => {
        if (disposed || !canvasRef.current) return;
        const viewport = page.getViewport({ scale: 1.35 });
        const canvas = canvasRef.current;
        const context = canvas.getContext("2d");
        if (!context) throw new Error("Canvas is unavailable");
        canvas.width = viewport.width;
        canvas.height = viewport.height;
        renderTask = page.render({ canvas, canvasContext: context, viewport });
        await renderTask.promise;
        if (!disposed) setLoading(false);
      })
      .catch(() => {
        if (!disposed) {
          setLoading(false);
          setError(true);
        }
      });
    return () => {
      disposed = true;
      renderTask?.cancel();
    };
  }, [documentReady, open, pageNumber]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="full" className="h-[calc(100dvh-2rem)] p-4 sm:p-5" aria-describedby="pdf-reader-description">
        <DialogHeader className="pr-10 text-start">
          <DialogTitle className="truncate">{title}</DialogTitle>
          <DialogDescription id="pdf-reader-description" className="sr-only">
            {t("knowledge.documents.reader-description")}
          </DialogDescription>
        </DialogHeader>

        <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-border bg-muted/40">
          <div className="flex min-h-12 flex-wrap items-center justify-between gap-2 border-b border-border bg-background px-2 py-1.5 sm:px-3">
            <div className="flex items-center gap-1">
              <Button
                variant="ghost"
                size="icon"
                className="size-11"
                disabled={loading || pageNumber <= 1}
                aria-label={t("knowledge.documents.previous-page")}
                onClick={() => setPageNumber((current) => Math.max(1, current - 1))}
              >
                <ChevronLeftIcon aria-hidden="true" />
              </Button>
              <span className="min-w-24 text-center text-sm tabular-nums text-foreground">
                {t("knowledge.documents.page-count", { page: pageNumber, count: pageCount || "-" })}
              </span>
              <Button
                variant="ghost"
                size="icon"
                className="size-11"
                disabled={loading || pageCount === 0 || pageNumber >= pageCount}
                aria-label={t("knowledge.documents.next-page")}
                onClick={() => setPageNumber((current) => Math.min(pageCount, current + 1))}
              >
                <ChevronRightIcon aria-hidden="true" />
              </Button>
            </div>
            <a href={url} download className={cn(buttonVariants({ variant: "outline" }), "h-11")}>
              <DownloadIcon aria-hidden="true" />
              {t("knowledge.documents.download")}
            </a>
          </div>

          <div className="relative min-h-0 flex-1 overflow-auto p-3 sm:p-5">
            {loading && (
              <div className="absolute inset-0 z-10 flex items-center justify-center bg-background/75" role="status">
                <LoaderCircleIcon className="size-5 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                <span className="ml-2 text-sm text-muted-foreground">{t("knowledge.documents.reader-loading")}</span>
              </div>
            )}
            {error ? (
              <div className="flex min-h-48 items-center justify-center text-center text-sm text-destructive" role="alert">
                {t("knowledge.documents.reader-error")}
              </div>
            ) : (
              <canvas
                ref={canvasRef}
                className="mx-auto block h-auto max-w-full bg-white shadow-sm"
                role="img"
                aria-label={t("knowledge.documents.page-label", { page: pageNumber })}
              />
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
};

export default PdfReaderDialog;
