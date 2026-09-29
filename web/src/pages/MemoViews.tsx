import { create } from "@bufbuild/protobuf";
import { FieldMaskSchema } from "@bufbuild/protobuf/wkt";
import { useQueryClient } from "@tanstack/react-query";
import {
  AstroidIcon,
  CheckCircle2Icon,
  ClipboardCheckIcon,
  Clock3Icon,
  ExternalLinkIcon,
  FilterIcon,
  MapPinIcon,
  MoreVerticalIcon,
  ParenthesesIcon,
  PencilIcon,
  PinIcon,
  PlusIcon,
  SaveIcon,
  SearchIcon,
  ShieldIcon,
  TagsIcon,
  Trash2Icon,
  XIcon,
} from "lucide-react";
import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useLocation, useNavigate } from "react-router-dom";
import ConfirmDialog from "@/components/ConfirmDialog";
import CustomIconPicker from "@/components/CustomIconPicker";
import MemoViewIcon from "@/components/MemoViewIcon";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { userServiceClient } from "@/connect";
import { useMemoFilterContext } from "@/contexts/MemoFilterContext";
import useCurrentUser from "@/hooks/useCurrentUser";
import useLoading from "@/hooks/useLoading";
import { useMemoViews, userKeys } from "@/hooks/useUserQueries";
import { handleError } from "@/lib/error";
import { getMemoViewId } from "@/lib/memo-views";
import { cn } from "@/lib/utils";
import { MemoView, MemoView_IconSchema, MemoViewSchema } from "@/types/proto/api/v1/user_service_pb";
import { useTranslate } from "@/utils/i18n";

const memoViewExamples = [
  {
    title: "置顶备忘录",
    filter: "pinned",
    description: "只显示已置顶的备忘录。",
    icon: PinIcon,
  },
  {
    title: "最近的备忘录",
    filter: 'created_ts >= now - duration("1h")',
    description: "显示最近一小时创建的备忘录。",
    icon: Clock3Icon,
  },
  {
    title: "公开备忘录",
    filter: 'visibility == "PUBLIC"',
    description: "只显示公开的备忘录。",
    icon: ShieldIcon,
  },
  {
    title: "项目标签",
    filter: 'tag in ["work", "personal"]',
    description: "匹配指定的一个或多个标签。",
    icon: TagsIcon,
  },
  {
    title: "无标签",
    filter: "size(tags) == 0",
    description: "显示没有标签的备忘录。",
    icon: TagsIcon,
  },
  {
    title: "归档标签树",
    filter: 'tags.exists(t, t.startsWith("archive"))',
    description: "按前缀匹配层级标签。",
    icon: TagsIcon,
  },
  {
    title: "未分类",
    filter: "space == null",
    description: "显示未放入任何空间的备忘录。",
    icon: AstroidIcon,
  },
  {
    title: "指定空间",
    filter: 'space == "spaces/your-space-id"',
    description: "显示指定空间中的备忘录。空间 ID 可从网址或设置中获取。",
    icon: AstroidIcon,
  },
  {
    title: "未完成任务",
    filter: "has_task_list && has_incomplete_tasks",
    description: "显示包含未完成任务的备忘录。",
    icon: ClipboardCheckIcon,
  },
  {
    title: "链接或代码",
    filter: "has_link || has_code",
    description: "显示包含链接或代码块的备忘录。",
    icon: FilterIcon,
  },
  {
    title: "无位置",
    filter: "!has_location",
    description: "显示未添加位置的备忘录。",
    icon: MapPinIcon,
  },
  {
    title: "内容搜索",
    filter: 'content.contains("TODO")',
    description: "搜索备忘录正文中的文字。",
    icon: SearchIcon,
  },
  {
    title: "以指定文字开头",
    filter: 'content.startsWith("TODO")',
    description: "显示正文以指定文字开头的备忘录，也可使用 endsWith 判断结尾。",
    icon: SearchIcon,
  },
  {
    title: "正则匹配",
    filter: 'content.matches("v[0-9]+")',
    description: "使用正则表达式匹配正文。",
    icon: FilterIcon,
  },
  {
    title: "全部完成的标签",
    filter: 'tags.all(t, t.endsWith("-done"))',
    description: "所有标签都以 -done 结尾，仅匹配有标签的备忘录。",
    icon: TagsIcon,
  },
  {
    title: "恰好一个项目标签",
    filter: 'tags.exists_one(t, t.startsWith("project/"))',
    description: "恰好只有一个标签符合条件。",
    icon: TagsIcon,
  },
  {
    title: "任意指定标签",
    filter: 'sets.intersects(tags, ["work", "urgent"])',
    description: "标签与指定集合有交集。",
    icon: TagsIcon,
  },
  {
    title: "完全匹配标签",
    filter: 'sets.equivalent(tags, ["inbox"])',
    description: "标签集合必须完全一致。",
    icon: TagsIcon,
  },
  {
    title: "2024 年的备忘录",
    filter: "created_ts.getFullYear() == 2024",
    description: "按自然年筛选。",
    icon: Clock3Icon,
  },
  {
    title: "周末的备忘录",
    filter: "created_ts.getDayOfWeek() == 0 || created_ts.getDayOfWeek() == 6",
    description: "显示周六或周日创建的备忘录（0 表示周日）。",
    icon: Clock3Icon,
  },
  {
    title: "长篇备忘录",
    filter: "size(content) > 280",
    description: "显示正文超过 280 字符的备忘录。",
    icon: FilterIcon,
  },
];

const filterFields = [
  "content.contains(...)",
  "content.startsWith(...)",
  "content.endsWith(...)",
  "content.matches(...)",
  "visibility",
  "pinned",
  "space == null",
  "space != null",
  'space == "spaces/..."',
  "tag in [...]",
  "tags.exists(...)",
  "tags.all(...)",
  "tags.exists_one(...)",
  "sets.contains(tags, [...])",
  "sets.intersects(tags, [...])",
  "sets.equivalent(tags, [...])",
  "size(tags) == ...",
  "size(content) > ...",
  "has_task_list",
  "has_incomplete_tasks",
  "has_link",
  "has_code",
  "has_location",
  'created_ts >= now - duration("24h")',
  "created_ts.getFullYear() == ...",
  "created_ts.getMonth() == ...（0 = 1 月）",
  "created_ts.getDayOfWeek() == ...（0 = 周日）",
  "updated_ts",
  "now",
  'timestamp("2025-01-01T00:00:00Z")',
];

const createEmptyMemoView = () =>
  create(MemoViewSchema, {
    name: "",
    title: "",
    filter: "",
  });

interface MemoViewGuideProps {
  onUseExample: (example: (typeof memoViewExamples)[number]) => void;
}

interface MemoViewsRouteState {
  openCreate?: boolean;
  memoView?: MemoView;
}

const MemoViewGuide = ({ onUseExample }: MemoViewGuideProps) => {
  return (
    <aside className="flex flex-col gap-5">
      <div className="rounded-lg border border-border p-4">
        <h2 className="text-sm font-semibold text-foreground">筛选表达式示例</h2>
        <div className="mt-3 flex flex-col gap-2">
          {memoViewExamples.map((example) => {
            const Icon = example.icon;
            return (
              <button
                type="button"
                key={example.filter}
                className="group w-full cursor-pointer rounded-md border border-transparent p-2 text-left transition-colors hover:border-border hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
                onClick={() => onUseExample(example)}
              >
                <span className="flex items-center gap-2 text-sm font-medium text-foreground">
                  <Icon className="h-4 w-4 text-muted-foreground group-hover:text-primary" />
                  {example.title}
                </span>
                <span className="mt-1 block font-mono text-xs leading-5 text-muted-foreground">{example.filter}</span>
                <span className="mt-1 block text-xs leading-5 text-muted-foreground">{example.description}</span>
              </button>
            );
          })}
        </div>
      </div>

      <div className="rounded-lg border border-border p-4">
        <h2 className="text-sm font-semibold text-foreground">支持的字段与写法</h2>
        <div className="mt-3 flex flex-wrap gap-2">
          {filterFields.map((field) => (
            <Badge key={field} variant="secondary" className="font-mono">
              {field}
            </Badge>
          ))}
        </div>
      </div>
    </aside>
  );
};

const MemoViews = () => {
  const t = useTranslate();
  const location = useLocation();
  const navigate = useNavigate();
  const user = useCurrentUser();
  const queryClient = useQueryClient();
  const { data: memoViews = [] } = useMemoViews(user?.name);
  const { memoView: selectedMemoView, setMemoView } = useMemoFilterContext();
  const [isCreateFormOpen, setIsCreateFormOpen] = useState(false);
  const [draft, setDraft] = useState<MemoView>(createEmptyMemoView());
  const [deleteTarget, setDeleteTarget] = useState<MemoView | undefined>();
  const createState = useLoading(false);
  const validateState = useLoading(false);
  const updateState = useLoading(false);
  const isEditing = draft.name !== "";
  const isSaving = createState.isLoading || updateState.isLoading;

  useEffect(() => {
    const state = location.state as MemoViewsRouteState | null;
    if (!state) return;

    if (state.memoView) {
      setDraft(
        create(MemoViewSchema, {
          name: state.memoView.name,
          title: state.memoView.title,
          filter: state.memoView.filter,
          icon: state.memoView.icon,
        }),
      );
      setIsCreateFormOpen(true);
    } else if (state.openCreate) {
      setDraft(createEmptyMemoView());
      setIsCreateFormOpen(true);
    }

    navigate(location.pathname, { replace: true, state: null });
  }, [location.key, location.pathname, location.state, navigate]);

  const setDraftState = (state: Partial<MemoView>) => {
    setDraft((current) => ({ ...current, ...state }));
  };

  const handleUseExample = (example: (typeof memoViewExamples)[number]) => {
    setDraft(
      create(MemoViewSchema, {
        name: draft.name,
        title: draft.title || example.title,
        filter: example.filter,
        icon: draft.icon,
      }),
    );
    setIsCreateFormOpen(true);
  };

  const handleOpenCreateForm = () => {
    setDraft(createEmptyMemoView());
    setIsCreateFormOpen(true);
  };

  const handleCloseForm = () => {
    setDraft(createEmptyMemoView());
    setIsCreateFormOpen(false);
  };

  const handleEditMemoView = (memoView: MemoView) => {
    setDraft(
      create(MemoViewSchema, {
        name: memoView.name,
        title: memoView.title,
        filter: memoView.filter,
        icon: memoView.icon,
      }),
    );
    setIsCreateFormOpen(true);
  };

  const validateDraft = async () => {
    if (!draft.title || !draft.filter) {
      toast.error("标题和筛选条件不能为空");
      return false;
    }
    if (!user?.name) {
      toast.error("当前未登录");
      return false;
    }

    try {
      validateState.setLoading();
      await userServiceClient.createMemoView({
        parent: user.name,
        memoView: { name: "", title: draft.title, filter: draft.filter, icon: draft.icon },
        validateOnly: true,
      });
      validateState.setFinish();
      toast.success("筛选表达式有效");
      return true;
    } catch (error: unknown) {
      await handleError(error, toast.error, {
        context: "Validate memo view filter",
        onError: () => validateState.setError(),
      });
      return false;
    }
  };

  const handleCreateMemoView = async () => {
    if (!draft.title || !draft.filter) {
      toast.error("标题和筛选条件不能为空");
      return;
    }
    if (!user?.name) {
      toast.error("当前未登录");
      return;
    }

    try {
      createState.setLoading();
      await userServiceClient.createMemoView({
        parent: user.name,
        memoView: { name: "", title: draft.title, filter: draft.filter, icon: draft.icon },
      });
      await queryClient.invalidateQueries({ queryKey: userKeys.memoViews(user.name) });
      createState.setFinish();
      setDraft(createEmptyMemoView());
      setIsCreateFormOpen(false);
      toast.success("视图已创建");
    } catch (error: unknown) {
      await handleError(error, toast.error, {
        context: "Create memo view",
        onError: () => createState.setError(),
      });
    }
  };

  const handleUpdateMemoView = async () => {
    if (!draft.title || !draft.filter) {
      toast.error("标题和筛选条件不能为空");
      return;
    }

    try {
      updateState.setLoading();
      await userServiceClient.updateMemoView({
        memoView: draft,
        updateMask: create(FieldMaskSchema, { paths: ["title", "filter", "icon"] }),
      });
      await queryClient.invalidateQueries({ queryKey: userKeys.memoViews(user?.name) });
      updateState.setFinish();
      setDraft(createEmptyMemoView());
      setIsCreateFormOpen(false);
      toast.success("视图已更新");
    } catch (error: unknown) {
      await handleError(error, toast.error, {
        context: "Update memo view",
        onError: () => updateState.setError(),
      });
    }
  };

  const handleSaveMemoView = async () => {
    if (isEditing) {
      await handleUpdateMemoView();
      return;
    }

    await handleCreateMemoView();
  };

  const confirmDeleteMemoView = async () => {
    if (!deleteTarget) return;

    try {
      await userServiceClient.deleteMemoView({ name: deleteTarget.name });
      await queryClient.invalidateQueries({ queryKey: userKeys.memoViews(user?.name) });
      if (selectedMemoView === getMemoViewId(deleteTarget.name)) setMemoView(undefined);
      toast.success(t("setting.memo-view.delete-success", { title: deleteTarget.title }));
    } catch (error: unknown) {
      await handleError(error, toast.error, {
        context: "Delete memo view",
      });
    } finally {
      setDeleteTarget(undefined);
    }
  };

  return (
    <section className="mx-auto flex w-full max-w-6xl flex-col gap-6 pb-10">
      <div className="flex flex-col gap-2 border-b border-border pb-5 sm:flex-row sm:items-end sm:justify-between">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-muted-foreground">
            <FilterIcon className="h-4 w-4" />
            <span className="text-sm font-medium">{t("common.views")}</span>
          </div>
          <h1 className="text-2xl font-semibold tracking-normal text-foreground">视图</h1>
          <p className="max-w-2xl text-sm leading-6 text-muted-foreground">
            使用字段、运算符、时间函数和标签条件创建可重复使用的视图。可以先选一个示例，再验证并保存。
          </p>
        </div>
        <Button onClick={isCreateFormOpen ? handleCloseForm : handleOpenCreateForm}>
          {isCreateFormOpen ? <XIcon className="h-4 w-4" /> : <PlusIcon className="h-4 w-4" />}
          {isCreateFormOpen ? t("common.cancel") : t("common.create")}
        </Button>
      </div>

      <div className={cn("grid grid-cols-1 gap-6", isCreateFormOpen && "xl:grid-cols-[minmax(0,1fr)_20rem]")}>
        <div className="flex min-w-0 flex-col gap-6">
          <div
            className={cn(
              "overflow-hidden rounded-lg border border-border bg-background transition-[max-height,opacity] duration-200",
              isCreateFormOpen ? "max-h-[48rem] opacity-100" : "max-h-0 border-transparent opacity-0",
            )}
          >
            <div className="grid gap-5 p-4 sm:p-5">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold text-foreground">{isEditing ? "编辑视图" : "创建视图"}</h2>
                  <p className="mt-1 text-sm text-muted-foreground">为视图命名，并填写用于筛选备忘录的表达式。</p>
                </div>
                <a
                  className="inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline"
                  href="https://www.usememos.com/docs"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  文档
                  <ExternalLinkIcon className="h-3.5 w-3.5" />
                </a>
              </div>

              <div className="grid gap-4">
                <div className="grid gap-2">
                  <Label htmlFor="view-title">{t("common.title")}</Label>
                  <div className="flex items-center gap-2">
                    <CustomIconPicker
                      value={draft.icon}
                      onChange={(icon) => setDraftState({ icon: icon ? create(MemoView_IconSchema, icon) : undefined })}
                      label={t("setting.memo-view.change-icon")}
                      fallback={ParenthesesIcon}
                      disabled={isSaving}
                    />
                    <Input
                      id="view-title"
                      value={draft.title}
                      placeholder="例如：置顶、最近、工作"
                      onChange={(event) => setDraftState({ title: event.target.value })}
                    />
                  </div>
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="view-filter">{t("common.filter")}</Label>
                  <Textarea
                    id="view-filter"
                    rows={5}
                    className="font-mono text-sm"
                    value={draft.filter}
                    placeholder='pinned && tag in ["work"]'
                    onChange={(event) => setDraftState({ filter: event.target.value })}
                  />
                  <p className="text-xs leading-5 text-muted-foreground">
                    用 <span className="font-mono">&&</span>、<span className="font-mono">||</span> 和 <span className="font-mono">!</span>
                    组合条件。时间字段是时间戳，可使用 <span className="font-mono">now</span>、
                    <span className="font-mono">duration("24h")</span>、<span className="font-mono">timestamp(...)</span> 和{" "}
                    <span className="font-mono">created_ts.getFullYear()</span>
                    等写法。标签支持 <span className="font-mono">sets.contains/intersects/equivalent</span>；
                    <span className="font-mono">size(content)</span> 用于计算正文长度。
                  </p>
                </div>
              </div>

              <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
                <Button variant="outline" disabled={validateState.isLoading || isSaving} onClick={validateDraft}>
                  <CheckCircle2Icon className="h-4 w-4" />
                  验证
                </Button>
                <Button disabled={isSaving || validateState.isLoading} onClick={handleSaveMemoView}>
                  <SaveIcon className="h-4 w-4" />
                  {t("common.save")}
                </Button>
              </div>
            </div>
          </div>

          <div className="flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <h2 className="text-base font-semibold text-foreground">全部视图</h2>
              <Badge variant="outline">{memoViews.length}</Badge>
            </div>

            {memoViews.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border px-4 py-10 text-center">
                <p className="text-sm font-medium text-foreground">还没有视图</p>
                <p className="mt-1 text-sm text-muted-foreground">打开创建表单，选择一个示例并添加第一个视图。</p>
              </div>
            ) : (
              <div className="divide-y divide-border overflow-hidden rounded-lg border border-border">
                {memoViews.map((memoView) => (
                  <div
                    key={memoView.name}
                    className="grid gap-3 bg-background px-4 py-3 sm:grid-cols-[minmax(10rem,14rem)_minmax(0,1fr)_2rem]"
                  >
                    <div className="min-w-0">
                      <div className="flex items-center gap-2 text-sm font-medium text-foreground">
                        <MemoViewIcon icon={memoView.icon} className="size-4 text-base" />
                        <span className="truncate">{memoView.title}</span>
                      </div>
                      <div className="mt-1 font-mono text-xs text-muted-foreground">{getMemoViewId(memoView.name)}</div>
                    </div>
                    <pre className="min-w-0 overflow-x-auto rounded-md bg-muted/50 px-3 py-2 font-mono text-xs leading-5 text-muted-foreground">
                      {memoView.filter}
                    </pre>
                    <DropdownMenu>
                      <DropdownMenuTrigger render={<Button variant="ghost" size="icon" className="justify-self-end" />}>
                        <MoreVerticalIcon className="h-4 w-4" />
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => handleEditMemoView(memoView)}>
                          <PencilIcon className="h-4 w-4" />
                          {t("common.edit")}
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => setDeleteTarget(memoView)}>
                          <Trash2Icon className="h-4 w-4" />
                          {t("common.delete")}
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {isCreateFormOpen ? <MemoViewGuide onUseExample={handleUseExample} /> : null}
      </div>

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(undefined)}
        title={t("setting.memo-view.delete-confirm", { title: deleteTarget?.title ?? "" })}
        confirmLabel={t("common.delete")}
        cancelLabel={t("common.cancel")}
        onConfirm={confirmDeleteMemoView}
        confirmVariant="destructive"
      />
    </section>
  );
};

export default MemoViews;
