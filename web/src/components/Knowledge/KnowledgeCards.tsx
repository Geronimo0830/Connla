import { timestampDate } from "@bufbuild/protobuf/wkt";
import { ArchiveIcon, LightbulbIcon, PlusIcon, SearchIcon } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "react-hot-toast";
import ReactMarkdown from "react-markdown";
import { Link, useSearchParams } from "react-router-dom";
import rehypeSanitize from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useKnowledgeCards, useSaveKnowledgeCard } from "@/hooks/useKnowledgeCardQueries";
import { useKnowledgeTopics } from "@/hooks/useKnowledgeTopicQueries";
import { ROUTES } from "@/router/routes";
import type { KnowledgeCard } from "@/types/proto/api/v1/knowledge_card_service_pb";
import CardEditorDialog from "./CardEditorDialog";

const typeLabels: Record<string, string> = {
  excerpt: "摘录",
  idea: "想法",
  question: "问题",
  summary: "总结",
  reference: "参考",
};

export default function KnowledgeCards() {
  const { data: cards = [], isLoading, isError, refetch } = useKnowledgeCards();
  const { data: topics = [] } = useKnowledgeTopics();
  const save = useSaveKnowledgeCard();
  const [searchParams, setSearchParams] = useSearchParams();
  const [query, setQuery] = useState("");
  const [showArchived, setShowArchived] = useState(false);
  const [newCardOpen, setNewCardOpen] = useState(false);
  const selectedName = searchParams.get("card");
  const selectedCard = cards.find((card) => card.name === selectedName);
  const activeCards = useMemo(
    () =>
      cards.filter(
        (card) =>
          (showArchived || !card.archived) && `${card.title} ${card.body}`.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()),
      ),
    [cards, query, showArchived],
  );

  const archive = async (card: KnowledgeCard) => {
    try {
      await save.mutateAsync({
        card,
        title: card.title,
        body: card.body,
        cardType: card.cardType,
        topics: card.topics,
        archived: !card.archived,
      });
      toast.success(card.archived ? "卡片已恢复" : "卡片已归档");
      setSearchParams({});
    } catch {
      toast.error("操作失败，请重试。");
    }
  };

  return (
    <div className="mt-8 space-y-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <h2 className="text-lg font-semibold">你的卡片</h2>
        <Button className="h-11" onClick={() => setNewCardOpen(true)}>
          <PlusIcon aria-hidden="true" />
          新建卡片
        </Button>
      </div>
      <div className="flex flex-col gap-2 sm:flex-row">
        <label className="relative flex-1">
          <span className="sr-only">搜索卡片</span>
          <SearchIcon className="pointer-events-none absolute left-3 top-3.5 size-4 text-muted-foreground" aria-hidden="true" />
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            className="min-h-11 pl-9 text-base"
            placeholder="搜索卡片"
          />
        </label>
        <Button
          variant={showArchived ? "secondary" : "outline"}
          className="h-11"
          aria-pressed={showArchived}
          onClick={() => setShowArchived((value) => !value)}
        >
          {showArchived ? "隐藏归档" : "查看归档"}
        </Button>
      </div>
      {isLoading ? (
        <p role="status" className="py-8 text-center text-sm text-muted-foreground">
          正在加载卡片…
        </p>
      ) : isError ? (
        <div role="alert" className="rounded-lg border p-5 text-sm">
          卡片暂时无法加载。
          <Button variant="outline" className="ml-3 h-11" onClick={() => refetch()}>
            重试
          </Button>
        </div>
      ) : activeCards.length === 0 ? (
        <div className="rounded-lg border border-dashed p-8 text-center">
          <LightbulbIcon className="mx-auto size-6 text-muted-foreground" aria-hidden="true" />
          <p className="mt-3 text-sm font-medium">{cards.length ? "没有符合条件的卡片" : "还没有知识卡片"}</p>
        </div>
      ) : (
        <ul className="space-y-3">
          {activeCards.map((card) => (
            <li key={card.name} className="rounded-lg border bg-background p-4 sm:p-5">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-xs text-muted-foreground">
                    {typeLabels[card.cardType] ?? "卡片"}
                    {card.archived ? " · 已归档" : ""}
                  </p>
                  <h3 className="mt-1 break-words text-base font-semibold">{card.title}</h3>
                </div>
                <Button variant="ghost" className="h-11 shrink-0" onClick={() => setSearchParams({ card: card.name })}>
                  编辑
                </Button>
              </div>
              <div className="prose prose-sm dark:prose-invert mt-2 max-w-none overflow-hidden text-sm leading-6">
                <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeSanitize]}>
                  {card.body}
                </ReactMarkdown>
              </div>
              {card.sourceQuote && (
                <details className="mt-3 rounded-md bg-muted/40 p-3 text-sm">
                  <summary className="cursor-pointer font-medium">查看原文依据 · {card.sourceTitle}</summary>
                  <blockquote className="mt-2 whitespace-pre-wrap border-l-2 border-border pl-3 leading-6">{card.sourceQuote}</blockquote>
                  {card.sourceAvailable ? (
                    <Link
                      to={`${ROUTES.KNOWLEDGE_DOCUMENTS}?document=${encodeURIComponent(card.sourceDocument)}&locator=${encodeURIComponent(card.sourceLocator)}`}
                      className="mt-2 inline-flex min-h-11 items-center text-primary underline"
                    >
                      返回原文位置
                    </Link>
                  ) : (
                    <p className="mt-2 text-xs text-muted-foreground">原文文档已移除，摘录快照仍保留。</p>
                  )}
                </details>
              )}
              <div className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t pt-2 text-xs text-muted-foreground">
                <span>
                  {card.topics
                    .map((name) => topics.find((topic) => topic.name === name)?.displayName)
                    .filter(Boolean)
                    .join(" · ") || "未分类"}
                  {card.updateTime ? ` · ${timestampDate(card.updateTime).toLocaleDateString("zh-CN")}` : ""}
                </span>
                <Button variant="ghost" className="h-11 text-xs" disabled={save.isPending} onClick={() => archive(card)}>
                  <ArchiveIcon className="size-4" aria-hidden="true" />
                  {card.archived ? "恢复" : "归档"}
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
      <CardEditorDialog open={newCardOpen} onOpenChange={setNewCardOpen} />
      {selectedCard && <CardEditorDialog open card={selectedCard} onOpenChange={(open) => !open && setSearchParams({})} />}
    </div>
  );
}
