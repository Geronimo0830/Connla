import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import KnowledgeSearch from "@/components/Knowledge/KnowledgeSearch";

const searchMocks = vi.hoisted(() => ({
  useKnowledgeSearch: vi.fn((..._args: [string, string, string, string, bigint, bigint, boolean]) => ({
    data: [],
    isFetching: false,
    isError: false,
    refetch: vi.fn(),
  })),
}));

vi.mock("@/hooks/useKnowledgeSearch", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/hooks/useKnowledgeSearch")>()),
  useKnowledgeSearch: searchMocks.useKnowledgeSearch,
  useRebuildKnowledgeSearch: () => ({ isPending: false, mutateAsync: vi.fn() }),
}));
vi.mock("@/hooks/useKnowledgeTopicQueries", () => ({
  useKnowledgeTopics: () => ({ data: [] }),
}));

describe("knowledge search filters", () => {
  it("passes type, format, and inclusive local date range to search", () => {
    render(
      <MemoryRouter>
        <KnowledgeSearch />
      </MemoryRouter>,
    );

    fireEvent.change(screen.getByLabelText("关键词"), { target: { value: "训练" } });
    fireEvent.change(screen.getByLabelText("内容类型"), { target: { value: "document" } });
    fireEvent.change(screen.getByLabelText("文件格式"), { target: { value: ".docx" } });
    fireEvent.click(screen.getByText("创建日期"));
    fireEvent.change(screen.getByLabelText("从"), { target: { value: "2026-09-01" } });
    fireEvent.change(screen.getByLabelText("到（含当天）"), { target: { value: "2026-09-29" } });

    expect(searchMocks.useKnowledgeSearch).toHaveBeenLastCalledWith(
      "训练",
      "",
      "document",
      ".docx",
      BigInt(Math.floor(new Date("2026-09-01T00:00:00").getTime() / 1000)),
      BigInt(Math.floor(new Date("2026-09-30T00:00:00").getTime() / 1000)),
      true,
    );
  });

  it("clears file format for cards and reports an invalid date range", () => {
    render(
      <MemoryRouter>
        <KnowledgeSearch />
      </MemoryRouter>,
    );
    fireEvent.change(screen.getByLabelText("文件格式"), { target: { value: ".pdf" } });
    fireEvent.change(screen.getByLabelText("内容类型"), { target: { value: "card" } });
    expect(screen.getByLabelText("文件格式")).toBeDisabled();
    expect(screen.getByLabelText("文件格式")).toHaveValue("");

    fireEvent.click(screen.getByText("创建日期"));
    fireEvent.change(screen.getByLabelText("从"), { target: { value: "2026-10-01" } });
    fireEvent.change(screen.getByLabelText("到（含当天）"), { target: { value: "2026-09-29" } });
    expect(screen.getByRole("alert")).toHaveTextContent("开始日期不能晚于结束日期");
    expect(searchMocks.useKnowledgeSearch.mock.lastCall?.[6]).toBe(false);
  });
});
