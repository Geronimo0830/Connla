import { FileTextIcon, LightbulbIcon, LoaderCircleIcon, RefreshCwIcon, SearchIcon } from "lucide-react";
import { Fragment, useDeferredValue, useState } from "react";
import { toast } from "react-hot-toast";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toSearchDateBound, useKnowledgeSearch, useRebuildKnowledgeSearch } from "@/hooks/useKnowledgeSearch";
import { useKnowledgeTopics } from "@/hooks/useKnowledgeTopicQueries";
import { ROUTES } from "@/router/routes";

const HighlightedSnippet = ({ text }: { text: string }) => (
  <>
    {text
      .split(/(<mark>|<\/mark>)/)
      .map((part, index) =>
        part === "<mark>" || part === "</mark>" ? null : (
          <Fragment key={`${index}-${part}`}>
            {text.split(/(<mark>|<\/mark>)/)[index - 1] === "<mark>" ? (
              <mark className="rounded bg-warning px-0.5 text-warning-foreground">{part}</mark>
            ) : (
              part
            )}
          </Fragment>
        ),
      )}
  </>
);

const KnowledgeSearch = () => {
  const [query, setQuery] = useState("");
  const [topic, setTopic] = useState("");
  const [contentType, setContentType] = useState("");
  const [fileFormat, setFileFormat] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const deferred = useDeferredValue(query.trim());
  const createdFrom = toSearchDateBound(startDate, false);
  const createdBefore = toSearchDateBound(endDate, true);
  const validRange = !createdFrom || !createdBefore || createdFrom < createdBefore;
  const { data: topics = [] } = useKnowledgeTopics();
  const {
    data: results = [],
    isFetching,
    isError,
    refetch,
  } = useKnowledgeSearch(deferred, topic, contentType, fileFormat, createdFrom, createdBefore, validRange);
  const rebuild = useRebuildKnowledgeSearch();
  return (
    <div className="mt-8 space-y-6">
      <section aria-labelledby="search-form-title">
        <div className="flex items-end justify-between gap-4">
          <h2 id="search-form-title" className="text-base font-semibold">
            搜索文档与卡片
          </h2>
          <Button
            variant="ghost"
            className="min-h-11"
            disabled={rebuild.isPending}
            onClick={async () => {
              try {
                const r = await rebuild.mutateAsync();
                toast.success(`索引已重建：${r.indexedDocuments} 个文档、${r.indexedCards} 张卡片`);
              } catch {
                toast.error("索引重建失败");
              }
            }}
          >
            <RefreshCwIcon className={`mr-2 size-4 ${rebuild.isPending ? "animate-spin" : ""}`} />
            修复索引
          </Button>
        </div>
        <div className="mt-5 grid gap-3 sm:grid-cols-[1fr_220px]">
          <label className="grid gap-1.5 text-sm font-medium" htmlFor="knowledge-search-query">
            关键词
            <div className="relative">
              <SearchIcon className="pointer-events-none absolute left-3 top-3.5 size-4 text-muted-foreground" />
              <Input
                id="knowledge-search-query"
                className="min-h-11 pl-9 text-base"
                value={query}
                maxLength={200}
                onChange={(e) => setQuery(e.target.value)}
              />
            </div>
          </label>
          <label className="grid gap-1.5 text-sm font-medium" htmlFor="knowledge-search-topic">
            主题
            <select
              id="knowledge-search-topic"
              className="min-h-11 rounded-md border bg-background px-3 text-base"
              value={topic}
              onChange={(e) => setTopic(e.target.value)}
            >
              <option value="">全部主题</option>
              {topics.map((t) => (
                <option key={t.name} value={t.name}>
                  {t.displayName}
                </option>
              ))}
            </select>
          </label>
        </div>
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          <label className="grid gap-1.5 text-sm font-medium" htmlFor="knowledge-search-type">
            内容类型
            <select
              id="knowledge-search-type"
              className="min-h-11 rounded-md border bg-background px-3 text-base"
              value={contentType}
              onChange={(event) => {
                setContentType(event.target.value);
                if (event.target.value === "card") setFileFormat("");
              }}
            >
              <option value="">全部内容</option>
              <option value="document">文档</option>
              <option value="card">知识卡片</option>
            </select>
          </label>
          <label className="grid gap-1.5 text-sm font-medium" htmlFor="knowledge-search-format">
            文件格式
            <select
              id="knowledge-search-format"
              className="min-h-11 rounded-md border bg-background px-3 text-base disabled:opacity-50"
              value={fileFormat}
              disabled={contentType === "card"}
              onChange={(event) => setFileFormat(event.target.value)}
            >
              <option value="">全部格式</option>
              {[".txt", ".md", ".markdown", ".pdf", ".doc", ".docx", ".xls", ".xlsx"].map((format) => (
                <option key={format} value={format}>
                  {format.slice(1).toUpperCase()}
                </option>
              ))}
            </select>
          </label>
        </div>
        <details className="mt-3 rounded-md border px-3 py-2">
          <summary className="cursor-pointer py-1 text-sm font-medium">创建日期</summary>
          <div className="mt-2 grid gap-3 pb-2 sm:grid-cols-2">
            <label className="grid gap-1 text-sm" htmlFor="knowledge-search-start-date">
              从
              <Input
                id="knowledge-search-start-date"
                type="date"
                className="min-h-11"
                value={startDate}
                onChange={(event) => setStartDate(event.target.value)}
              />
            </label>
            <label className="grid gap-1 text-sm" htmlFor="knowledge-search-end-date">
              到（含当天）
              <Input
                id="knowledge-search-end-date"
                type="date"
                className="min-h-11"
                value={endDate}
                onChange={(event) => setEndDate(event.target.value)}
              />
            </label>
          </div>
          {!validRange && (
            <p className="pb-2 text-sm text-destructive" role="alert">
              开始日期不能晚于结束日期。
            </p>
          )}
        </details>
      </section>
      <section aria-live="polite" aria-busy={isFetching}>
        {isFetching && (
          <p className="flex items-center gap-2 py-6 text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            正在搜索…
          </p>
        )}
        {isError && (
          <div className="rounded-lg border border-destructive/40 p-4">
            <p className="text-sm">搜索暂时不可用。</p>
            <Button variant="outline" className="mt-3" onClick={() => refetch()}>
              重试
            </Button>
          </div>
        )}
        {!isFetching && !isError && deferred && results.length === 0 && (
          <div className="rounded-lg border border-dashed p-8 text-center">
            <SearchIcon className="mx-auto size-6 text-muted-foreground" />
            <p className="mt-3 text-sm font-medium">没有找到相关内容</p>
          </div>
        )}
        {!isFetching && results.length > 0 && (
          <div>
            <p className="mb-3 text-sm text-muted-foreground">找到 {results.length} 条结果</p>
            <div className="space-y-3">
              {results.map((r) => (
                <article key={r.document || r.card} className="rounded-lg border bg-background p-4">
                  <div className="flex gap-3">
                    {r.card ? (
                      <LightbulbIcon className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
                    ) : (
                      <FileTextIcon className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
                    )}
                    <div className="min-w-0">
                      <h3 className="font-medium">{r.title || r.originalFilename}</h3>
                      <p className="mt-2 line-clamp-3 text-sm leading-6 text-muted-foreground">
                        <HighlightedSnippet text={r.snippet} />
                      </p>
                      <Link
                        to={r.card ? `${ROUTES.KNOWLEDGE_CARDS}?card=${encodeURIComponent(r.card)}` : ROUTES.KNOWLEDGE_DOCUMENTS}
                        className="mt-3 inline-flex min-h-11 items-center text-sm font-medium text-primary hover:underline"
                      >
                        {r.card ? "查看知识卡片" : "在文档收件箱中查看"}
                      </Link>
                    </div>
                  </div>
                </article>
              ))}
            </div>
          </div>
        )}
      </section>
    </div>
  );
};
export default KnowledgeSearch;
