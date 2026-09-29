import { BookOpenIcon, FileTextIcon, LightbulbIcon, type LucideIcon, SearchIcon, TagsIcon } from "lucide-react";
import { Link, matchPath, useLocation } from "react-router-dom";
import { useAppSidebar } from "@/contexts/AppSidebarContext";
import { cn } from "@/lib/utils";
import { ROUTES } from "@/router/routes";
import type { Translations } from "@/utils/i18n";
import { useTranslate } from "@/utils/i18n";
import { SIDEBAR_ROW_CLASSES, SidebarRowIconSlot, sidebarRowStateClasses } from "./AppSidebar/SidebarRow";
import SidebarSection from "./AppSidebar/SidebarSection";

export type KnowledgeModuleId = "documents" | "cards" | "topics" | "search";

export interface KnowledgeModuleDefinition {
  id: KnowledgeModuleId;
  path: string;
  icon: LucideIcon;
  labelKey: Translations;
  descriptionKey: Translations;
}

export const KNOWLEDGE_MODULES: KnowledgeModuleDefinition[] = [
  {
    id: "documents",
    path: ROUTES.KNOWLEDGE_DOCUMENTS,
    icon: FileTextIcon,
    labelKey: "knowledge.modules.documents.title",
    descriptionKey: "knowledge.modules.documents.description",
  },
  {
    id: "cards",
    path: ROUTES.KNOWLEDGE_CARDS,
    icon: LightbulbIcon,
    labelKey: "knowledge.modules.cards.title",
    descriptionKey: "knowledge.modules.cards.description",
  },
  {
    id: "topics",
    path: ROUTES.KNOWLEDGE_TOPICS,
    icon: TagsIcon,
    labelKey: "knowledge.modules.topics.title",
    descriptionKey: "knowledge.modules.topics.description",
  },
  {
    id: "search",
    path: ROUTES.KNOWLEDGE_SEARCH,
    icon: SearchIcon,
    labelKey: "knowledge.modules.search.title",
    descriptionKey: "knowledge.modules.search.description",
  },
];

export const KnowledgeSidebarContent = () => {
  const t = useTranslate();
  const location = useLocation();
  const { setMobileOpen } = useAppSidebar();

  return (
    <SidebarSection label={t("knowledge.title")}>
      <Link
        to={ROUTES.KNOWLEDGE}
        onClick={() => setMobileOpen(false)}
        className={cn(SIDEBAR_ROW_CLASSES, sidebarRowStateClasses(location.pathname === ROUTES.KNOWLEDGE ? "current" : "idle"))}
        aria-current={location.pathname === ROUTES.KNOWLEDGE ? "page" : undefined}
      >
        <SidebarRowIconSlot icon={BookOpenIcon} />
        <span className="truncate">{t("knowledge.overview")}</span>
      </Link>
      {KNOWLEDGE_MODULES.map((module) => {
        const active = matchPath({ path: module.path, caseSensitive: false, end: true }, location.pathname) !== null;
        return (
          <Link
            key={module.id}
            to={module.path}
            onClick={() => setMobileOpen(false)}
            className={cn(SIDEBAR_ROW_CLASSES, sidebarRowStateClasses(active ? "current" : "idle"))}
            aria-current={active ? "page" : undefined}
          >
            <SidebarRowIconSlot icon={module.icon} />
            <span className="truncate">{t(module.labelKey)}</span>
          </Link>
        );
      })}
    </SidebarSection>
  );
};
