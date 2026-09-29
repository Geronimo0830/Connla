import { ChevronRightIcon, FolderTreeIcon, LoaderCircleIcon, PencilIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "react-hot-toast";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useDocuments } from "@/hooks/useDocumentQueries";
import {
  useDeleteKnowledgeTopic,
  useDocumentTopics,
  useKnowledgeTopics,
  useSaveKnowledgeTopic,
  useSetDocumentTopics,
} from "@/hooks/useKnowledgeTopicQueries";
import type { KnowledgeTopic } from "@/types/proto/api/v1/knowledge_topic_service_pb";

const TopicManager = () => {
  const { data: topics = [], isLoading } = useKnowledgeTopics();
  const { data: documents = [] } = useDocuments();
  const save = useSaveKnowledgeTopic();
  const remove = useDeleteKnowledgeTopic();
  const [editing, setEditing] = useState<KnowledgeTopic>();
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [parent, setParent] = useState("");
  const roots = useMemo(() => topics.filter((t) => !t.parent), [topics]);
  const begin = (topic?: KnowledgeTopic) => {
    setEditing(topic);
    setCreating(true);
    setName(topic?.displayName ?? "");
    setDescription(topic?.description ?? "");
    setParent(topic?.parent ?? "");
  };
  const submit = async () => {
    if (!name.trim()) return;
    try {
      await save.mutateAsync({ name: editing?.name, displayName: name.trim(), description, parent });
      setCreating(false);
      toast.success("主题已保存");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "主题保存失败");
    }
  };
  const render = (topic: KnowledgeTopic, depth = 0) => (
    <div key={topic.name}>
      <div className="flex min-h-12 items-center gap-2 rounded-md px-2 hover:bg-muted/50" style={{ paddingLeft: `${8 + depth * 20}px` }}>
        <ChevronRightIcon className="size-4 text-muted-foreground" aria-hidden="true" />
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium">{topic.displayName}</span>
          {topic.description && <span className="block truncate text-xs text-muted-foreground">{topic.description}</span>}
        </span>
        <Button variant="ghost" size="icon" className="size-11" aria-label={`编辑${topic.displayName}`} onClick={() => begin(topic)}>
          <PencilIcon className="size-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="size-11 text-destructive"
          aria-label={`删除${topic.displayName}`}
          onClick={async () => {
            if (!confirm(`删除主题“${topic.displayName}”？文档不会被删除。`)) return;
            try {
              await remove.mutateAsync(topic.name);
              toast.success("主题已删除");
            } catch (e) {
              toast.error(e instanceof Error ? e.message : "请先处理子主题");
            }
          }}
        >
          <Trash2Icon className="size-4" />
        </Button>
      </div>
      {topics.filter((t) => t.parent === topic.name).map((t) => render(t, depth + 1))}
    </div>
  );
  return (
    <div className="mt-8 space-y-6">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-base font-semibold">主题结构</h2>
        <Button className="min-h-11" onClick={() => begin()}>
          <PlusIcon className="mr-2 size-4" />
          新建主题
        </Button>
      </div>
      {creating && (
        <section className="rounded-lg border bg-muted/20 p-4" aria-label="主题表单">
          <div className="grid gap-4">
            <label className="grid gap-1.5 text-sm font-medium">
              名称
              <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={80} />
            </label>
            <label className="grid gap-1.5 text-sm font-medium">
              说明
              <Input value={description} onChange={(e) => setDescription(e.target.value)} maxLength={300} />
            </label>
            <label className="grid gap-1.5 text-sm font-medium">
              父主题
              <select className="min-h-11 rounded-md border bg-background px-3" value={parent} onChange={(e) => setParent(e.target.value)}>
                <option value="">无（顶级主题）</option>
                {topics
                  .filter((t) => t.name !== editing?.name)
                  .map((t) => (
                    <option key={t.name} value={t.name}>
                      {t.displayName}
                    </option>
                  ))}
              </select>
            </label>
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={() => setCreating(false)}>
                取消
              </Button>
              <Button disabled={!name.trim() || save.isPending} onClick={submit}>
                {save.isPending && <LoaderCircleIcon className="mr-2 size-4 animate-spin" />}保存
              </Button>
            </div>
          </div>
        </section>
      )}
      <section className="rounded-lg border bg-background p-2">
        {isLoading ? (
          <p className="p-4 text-sm text-muted-foreground">正在加载…</p>
        ) : roots.length ? (
          roots.map((t) => render(t))
        ) : (
          <div className="p-6 text-center">
            <FolderTreeIcon className="mx-auto size-6 text-muted-foreground" />
            <p className="mt-2 text-sm">还没有主题</p>
          </div>
        )}
      </section>
      <section>
        <h2 className="text-base font-semibold">文档分类</h2>
        <div className="mt-3 space-y-2">
          {documents.map((d) => (
            <DocumentTopicRow key={d.name} document={d.name} title={d.title || d.originalFilename} topics={topics} />
          ))}
        </div>
      </section>
    </div>
  );
};

const DocumentTopicRow = ({ document, title, topics }: { document: string; title: string; topics: KnowledgeTopic[] }) => {
  const { data: selected = [] } = useDocumentTopics(document);
  const save = useSetDocumentTopics();
  const values = new Set(selected.map((t) => t.name));
  return (
    <details className="rounded-lg border bg-background">
      <summary className="cursor-pointer px-4 py-3 text-sm font-medium">
        {title}
        <span className="ml-2 text-xs font-normal text-muted-foreground">{selected.length} 个主题</span>
      </summary>
      <div className="grid gap-2 border-t p-4 sm:grid-cols-2">
        {topics.map((t) => (
          <label key={t.name} className="flex min-h-11 items-center gap-3 rounded-md px-2 hover:bg-muted/50">
            <input
              type="checkbox"
              checked={values.has(t.name)}
              disabled={save.isPending}
              onChange={async (e) => {
                const next = new Set(values);
                e.target.checked ? next.add(t.name) : next.delete(t.name);
                try {
                  await save.mutateAsync({ document, topics: [...next] });
                } catch {
                  toast.error("文档分类保存失败");
                }
              }}
            />
            <span className="text-sm">{t.displayName}</span>
          </label>
        ))}
      </div>
    </details>
  );
};
export default TopicManager;
