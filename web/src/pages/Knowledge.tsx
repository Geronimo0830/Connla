import { ArrowLeftIcon, ArrowRightIcon, BookOpenIcon } from "lucide-react";
import { Link } from "react-router-dom";
import DocumentInbox from "@/components/Knowledge/DocumentInbox";
import KnowledgeCards from "@/components/Knowledge/KnowledgeCards";
import KnowledgeSearch from "@/components/Knowledge/KnowledgeSearch";
import TopicManager from "@/components/Knowledge/TopicManager";
import { KNOWLEDGE_MODULES, type KnowledgeModuleId } from "@/components/KnowledgeNavigation";
import { ROUTES } from "@/router/routes";
import { useTranslate } from "@/utils/i18n";

interface Props {
  moduleId?: KnowledgeModuleId;
}

const KnowledgeOverview = () => {
  const t = useTranslate();

  return (
    <section className="mx-auto w-full max-w-4xl py-4 sm:py-8" aria-labelledby="knowledge-title">
      <header>
        <div className="flex size-10 items-center justify-center rounded-lg border border-border bg-muted/50" aria-hidden="true">
          <BookOpenIcon className="size-5 text-foreground" strokeWidth={1.8} />
        </div>
        <h1 id="knowledge-title" className="mt-4 text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
          {t("knowledge.title")}
        </h1>
      </header>

      <nav className="mt-8 grid grid-cols-1 gap-3 sm:grid-cols-2" aria-label={t("knowledge.modules-label")}>
        {KNOWLEDGE_MODULES.map((module) => {
          const Icon = module.icon;
          return (
            <Link
              key={module.id}
              to={module.path}
              className="group flex min-h-32 items-start gap-4 rounded-lg border border-border bg-background p-4 transition-colors duration-200 hover:border-foreground/20 hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background motion-reduce:transition-none"
            >
              <span
                className="flex size-10 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground group-hover:text-foreground"
                aria-hidden="true"
              >
                <Icon className="size-5" strokeWidth={1.8} />
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex items-center justify-between gap-3">
                  <span className="text-base font-medium text-foreground">{t(module.labelKey)}</span>
                  <ArrowRightIcon className="size-4 shrink-0 text-muted-foreground" strokeWidth={1.8} />
                </span>
                <span className="mt-1.5 block text-sm leading-6 text-muted-foreground">{t(module.descriptionKey)}</span>
              </span>
            </Link>
          );
        })}
      </nav>
    </section>
  );
};

const KnowledgeModule = ({ moduleId }: { moduleId: KnowledgeModuleId }) => {
  const t = useTranslate();
  const module = KNOWLEDGE_MODULES.find((item) => item.id === moduleId);
  if (!module) return null;

  const Icon = module.icon;
  return (
    <section className="mx-auto w-full max-w-3xl py-4 sm:py-8" aria-labelledby="knowledge-module-title">
      <Link
        to={ROUTES.KNOWLEDGE}
        className="inline-flex min-h-11 items-center gap-2 rounded-md px-2 text-sm text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <ArrowLeftIcon className="size-4" strokeWidth={1.8} aria-hidden="true" />
        {t("knowledge.back")}
      </Link>

      <div className="mt-5 flex items-start gap-4">
        <span className="flex size-11 shrink-0 items-center justify-center rounded-lg border border-border bg-muted/50" aria-hidden="true">
          <Icon className="size-5 text-foreground" strokeWidth={1.8} />
        </span>
        <div className="min-w-0">
          <h1 id="knowledge-module-title" className="text-2xl font-semibold tracking-tight text-foreground">
            {t(module.labelKey)}
          </h1>
        </div>
      </div>

      {moduleId === "documents" ? (
        <DocumentInbox />
      ) : moduleId === "cards" ? (
        <KnowledgeCards />
      ) : moduleId === "topics" ? (
        <TopicManager />
      ) : (
        <KnowledgeSearch />
      )}
    </section>
  );
};

const Knowledge = ({ moduleId }: Props) => (moduleId ? <KnowledgeModule moduleId={moduleId} /> : <KnowledgeOverview />);

export default Knowledge;
