import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import DocumentTextReader from "@/components/Knowledge/DocumentTextReader";
import KnowledgeCards from "@/components/Knowledge/KnowledgeCards";

const mocks = vi.hoisted(() => ({
  cards: [
    {
      name: "knowledge/cards/one",
      title: "安全的想法",
      body: "<script>alert('x')</script>**我的理解**",
      cardType: "idea",
      archived: false,
      topics: [],
      sourceDocument: "documents/source",
      sourceTitle: "来源文档",
      sourceQuote: "一段原文",
      sourceLocator: "text:1:5",
      sourceAvailable: true,
    },
  ],
  save: vi.fn(async () => ({})),
  editorSource: undefined as unknown,
}));

vi.mock("@/hooks/useKnowledgeCardQueries", () => ({
  useKnowledgeCards: () => ({ data: mocks.cards, isLoading: false, isError: false }),
  useSaveKnowledgeCard: () => ({ mutateAsync: mocks.save, isPending: false }),
}));
vi.mock("@/hooks/useKnowledgeTopicQueries", () => ({ useKnowledgeTopics: () => ({ data: [] }) }));
vi.mock("@/hooks/useDocumentQueries", () => ({
  useDocumentContent: () => ({ data: { plainText: "学😀卡片", contentHash: "hash" }, isLoading: false, isError: false }),
}));
vi.mock("@/components/Knowledge/CardEditorDialog", () => ({
  default: ({ open, source }: { open: boolean; source?: unknown }) => {
    if (open) mocks.editorSource = source;
    return open ? <div data-testid="card-editor">卡片编辑器</div> : null;
  },
}));

describe("Knowledge cards", () => {
  it("shows source evidence and sanitizes Markdown", () => {
    const { container } = render(
      <MemoryRouter>
        <KnowledgeCards />
      </MemoryRouter>,
    );
    expect(screen.getByText("安全的想法")).toBeInTheDocument();
    expect(screen.queryByText(/用自己的话记录一个想法/)).not.toBeInTheDocument();
    expect(container.querySelector("script")).toBeNull();
    fireEvent.click(screen.getByText(/查看原文依据/));
    expect(screen.getByText("一段原文")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "返回原文位置" })).toHaveAttribute(
      "href",
      "/knowledge/documents?document=documents%2Fsource&locator=text%3A1%3A5",
    );
  });

  it("counts Unicode code points when selecting exact source text", () => {
    render(
      <MemoryRouter>
        <DocumentTextReader
          document={{ name: "documents/source", title: "来源文档", originalFilename: "source.txt" } as never}
          onClose={vi.fn()}
        />
      </MemoryRouter>,
    );
    const root = screen.getByText("学😀卡片");
    const text = root.firstChild;
    expect(text).toBeTruthy();
    const range = document.createRange();
    range.setStart(text!, 1);
    range.setEnd(text!, 4);
    window.getSelection()?.removeAllRanges();
    window.getSelection()?.addRange(range);
    fireEvent.mouseUp(root);
    expect(screen.getByText("已选中 2 字")).toBeInTheDocument();
    expect(screen.queryByText(/这里是可替换的解析文本/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "制成卡片" }));
    expect(mocks.editorSource).toMatchObject({ startOffset: 1, endOffset: 3, quote: "😀卡" });
  });
});
