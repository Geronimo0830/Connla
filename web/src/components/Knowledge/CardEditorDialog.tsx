import { useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import ReactMarkdown from "react-markdown";
import rehypeSanitize from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { type CardSourceSelection, useSaveKnowledgeCard } from "@/hooks/useKnowledgeCardQueries";
import { useKnowledgeTopics } from "@/hooks/useKnowledgeTopicQueries";
import type { KnowledgeCard } from "@/types/proto/api/v1/knowledge_card_service_pb";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  card?: KnowledgeCard;
  source?: CardSourceSelection;
}

const cardTypes = [
  { value: "idea", label: "想法" },
  { value: "excerpt", label: "摘录" },
  { value: "question", label: "问题" },
  { value: "summary", label: "总结" },
  { value: "reference", label: "参考" },
];

export default function CardEditorDialog({ open, onOpenChange, card, source }: Props) {
  const { data: topics = [] } = useKnowledgeTopics();
  const save = useSaveKnowledgeCard();
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [cardType, setCardType] = useState("idea");
  const [selectedTopics, setSelectedTopics] = useState<string[]>([]);
  const [preview, setPreview] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) return;
    setTitle(card?.title ?? "");
    setBody(card?.body ?? "");
    setCardType(card?.cardType ?? (source ? "excerpt" : "idea"));
    setSelectedTopics(card?.topics ?? []);
    setPreview(false);
    setError("");
  }, [open, card, source]);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!title.trim() || !body.trim()) {
      setError("请填写标题和自己的理解。");
      return;
    }
    try {
      await save.mutateAsync({ card, title: title.trim(), body: body.trim(), cardType, topics: selectedTopics, source });
      toast.success(card ? "卡片已更新" : "卡片已保存");
      onOpenChange(false);
    } catch {
      setError("保存失败。请检查内容或来源是否已重新解析，然后重试。");
    }
  };

  const sourceQuote = card?.sourceQuote || source?.quote;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto" aria-describedby="card-editor-description">
        <DialogHeader>
          <DialogTitle>{card ? "编辑知识卡片" : "新建知识卡片"}</DialogTitle>
          <DialogDescription id="card-editor-description" className="sr-only">
            填写标题、类型和内容，保存知识卡片。
          </DialogDescription>
        </DialogHeader>
        <form className="space-y-4" onSubmit={submit}>
          {sourceQuote && (
            <div className="rounded-lg border bg-muted/40 p-3">
              <p className="text-xs font-medium text-muted-foreground">原文依据 · {card?.sourceTitle || source?.title}</p>
              <blockquote className="mt-2 max-h-32 overflow-auto whitespace-pre-wrap border-l-2 border-border pl-3 text-sm leading-6">
                {sourceQuote}
              </blockquote>
            </div>
          )}
          <label className="block space-y-1.5 text-sm font-medium" htmlFor="knowledge-card-title">
            标题
            <Input
              id="knowledge-card-title"
              className="min-h-11 text-base"
              value={title}
              maxLength={200}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="这张卡片要记住什么？"
            />
          </label>
          <label className="block space-y-1.5 text-sm font-medium" htmlFor="knowledge-card-type">
            类型
            <select
              id="knowledge-card-type"
              className="flex min-h-11 w-full rounded-md border bg-background px-3 text-base"
              value={cardType}
              onChange={(e) => setCardType(e.target.value)}
            >
              {cardTypes.map((type) => (
                <option key={type.value} value={type.value}>
                  {type.label}
                </option>
              ))}
            </select>
          </label>
          <div>
            <div className="flex items-center justify-between gap-2">
              <label className="text-sm font-medium" htmlFor="knowledge-card-body">
                自己的理解（支持 Markdown）
              </label>
              <Button type="button" variant="ghost" className="h-11" aria-pressed={preview} onClick={() => setPreview((value) => !value)}>
                {preview ? "继续编辑" : "预览"}
              </Button>
            </div>
            {preview ? (
              <div className="prose prose-sm dark:prose-invert min-h-32 max-w-none rounded-md border p-3" aria-label="卡片预览">
                <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeSanitize]}>
                  {body}
                </ReactMarkdown>
              </div>
            ) : (
              <textarea
                id="knowledge-card-body"
                className="min-h-36 w-full resize-y rounded-md border bg-background p-3 text-base leading-6 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                value={body}
                maxLength={20000}
                onChange={(e) => setBody(e.target.value)}
              />
            )}
          </div>
          {topics.length > 0 && (
            <fieldset className="space-y-2">
              <legend className="text-sm font-medium">学习主题（可选）</legend>
              <div className="flex max-h-28 flex-wrap gap-2 overflow-y-auto">
                {topics.map((topic) => (
                  <label key={topic.name} className="inline-flex min-h-11 items-center gap-2 rounded-md border px-3 text-sm">
                    <input
                      type="checkbox"
                      checked={selectedTopics.includes(topic.name)}
                      onChange={(event) =>
                        setSelectedTopics((current) =>
                          event.target.checked ? [...current, topic.name] : current.filter((name) => name !== topic.name),
                        )
                      }
                    />
                    {topic.displayName}
                  </label>
                ))}
              </div>
            </fieldset>
          )}
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" className="h-11" onClick={() => onOpenChange(false)}>
              取消
            </Button>
            <Button type="submit" className="h-11" disabled={save.isPending}>
              {save.isPending ? "正在保存…" : "保存卡片"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
