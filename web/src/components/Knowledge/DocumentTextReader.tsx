import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useDocumentContent } from "@/hooks/useDocumentQueries";
import type { CardSourceSelection } from "@/hooks/useKnowledgeCardQueries";
import type { Document } from "@/types/proto/api/v1/document_service_pb";
import CardEditorDialog from "./CardEditorDialog";

interface Props {
  document: Document;
  locator?: string;
  onClose: () => void;
}

function parseLocator(locator: string | undefined, max: number) {
  const match = /^text:(\d+):(\d+)$/.exec(locator ?? "");
  if (!match) return undefined;
  const start = Number(match[1]);
  const end = Number(match[2]);
  return start >= 0 && end > start && end <= max ? { start, end } : undefined;
}

export default function DocumentTextReader({ document, locator, onClose }: Props) {
  const { data: content, isLoading, isError, refetch } = useDocumentContent(document.name);
  const textRef = useRef<HTMLDivElement>(null);
  const markRef = useRef<HTMLElement>(null);
  const [selection, setSelection] = useState<CardSourceSelection>();
  const [selectionHint, setSelectionHint] = useState("");
  const [cardOpen, setCardOpen] = useState(false);
  const text = content?.plainText ?? "";
  const runes = Array.from(text);
  const marked = parseLocator(locator, runes.length);

  useEffect(() => {
    if (content && marked) markRef.current?.scrollIntoView({ block: "center" });
  }, [content, locator]);

  const updateSelection = () => {
    const root = textRef.current;
    const range = window.getSelection()?.rangeCount ? window.getSelection()?.getRangeAt(0) : undefined;
    if (!root || !content || !range || !root.contains(range.startContainer) || !root.contains(range.endContainer)) {
      setSelection(undefined);
      setSelectionHint("");
      return;
    }
    const startRange = range.cloneRange();
    startRange.selectNodeContents(root);
    startRange.setEnd(range.startContainer, range.startOffset);
    const endRange = range.cloneRange();
    endRange.selectNodeContents(root);
    endRange.setEnd(range.endContainer, range.endOffset);
    const start = Array.from(text.slice(0, startRange.toString().length)).length;
    const end = Array.from(text.slice(0, endRange.toString().length)).length;
    if (end <= start || end - start > 2000) {
      setSelection(undefined);
      setSelectionHint(end - start > 2000 ? "一次最多选择 2000 字，请缩小范围。" : "");
      return;
    }
    setSelectionHint("");
    setSelection({
      document: document.name,
      contentHash: content.contentHash,
      startOffset: start,
      endOffset: end,
      quote: runes.slice(start, end).join(""),
      title: document.title || document.originalFilename,
    });
  };

  return (
    <>
      <Dialog open onOpenChange={(open) => !open && onClose()}>
        <DialogContent size="full" className="h-[calc(100dvh-2rem)] p-4 sm:p-5" aria-describedby="text-reader-description">
          <DialogHeader className="pr-10 text-start">
            <DialogTitle>{document.title || document.originalFilename}</DialogTitle>
            <DialogDescription id="text-reader-description" className="sr-only">
              阅读文档解析文本
            </DialogDescription>
          </DialogHeader>
          <div className="flex min-h-0 flex-1 flex-col rounded-lg border">
            <div className="flex min-h-14 flex-wrap items-center justify-between gap-2 border-b p-2">
              <span className="text-sm text-muted-foreground" role="status">
                {selectionHint || (selection ? `已选中 ${selection.endOffset - selection.startOffset} 字` : "请在下方选中一段文字")}
              </span>
              <Button className="h-11" disabled={!selection} onClick={() => setCardOpen(true)}>
                制成卡片
              </Button>
            </div>
            <div className="min-h-0 flex-1 overflow-auto p-4 sm:p-6">
              {isLoading ? (
                <p role="status" className="text-sm">
                  正在读取文本…
                </p>
              ) : isError ? (
                <div role="alert" className="text-sm">
                  暂时无法读取解析文本。
                  <Button variant="outline" className="ml-2 h-11" onClick={() => refetch()}>
                    重试
                  </Button>
                </div>
              ) : !text ? (
                <p className="text-sm text-muted-foreground">没有可选择的文本。</p>
              ) : (
                <div
                  ref={textRef}
                  onMouseUp={updateSelection}
                  onKeyUp={updateSelection}
                  className="mx-auto max-w-3xl select-text whitespace-pre-wrap break-words text-base leading-8 text-foreground"
                >
                  {marked ? (
                    <>
                      {runes.slice(0, marked.start).join("")}
                      <mark ref={markRef} className="rounded bg-warning text-warning-foreground">
                        {runes.slice(marked.start, marked.end).join("")}
                      </mark>
                      {runes.slice(marked.end).join("")}
                    </>
                  ) : (
                    text
                  )}
                </div>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>
      {cardOpen && selection && <CardEditorDialog open source={selection} onOpenChange={(open) => !open && setCardOpen(false)} />}
    </>
  );
}
