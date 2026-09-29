import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import Knowledge from "@/pages/Knowledge";

const documentMocks = vi.hoisted(() => ({
  documents: [
    {
      name: "documents/ready",
      title: "Learning notes",
      originalFilename: "notes.md",
      extension: ".md",
      sourceAttachment: "attachments/a",
      size: 120n,
      status: 4,
    },
    {
      name: "documents/failed",
      title: "Broken text",
      originalFilename: "broken.txt",
      extension: ".txt",
      sourceAttachment: "attachments/b",
      size: 8n,
      status: 5,
      errorMessage: "The document is not valid UTF-8 text",
    },
    {
      name: "documents/pdf",
      title: "Reference",
      originalFilename: "reference.pdf",
      extension: ".pdf",
      sourceAttachment: "attachments/c",
      size: 2048n,
      status: 6,
      errorMessage: "This document format is not supported yet",
    },
    {
      name: "documents/paper",
      title: "Readable paper",
      originalFilename: "paper.pdf",
      extension: ".pdf",
      sourceAttachment: "attachments/d",
      size: 4096n,
      status: 4,
    },
  ],
  createDocument: vi.fn(async () => ({})),
  uploadFile: vi.fn(async (_localFile: { file: File }) => ({ name: "attachments/uploaded" })),
  getDeletePlan: vi.fn(async () => ({ derivedContentRows: 1, parseAttemptRows: 2, linkedCardCount: 0, sourceAttachmentRetained: true })),
}));

vi.mock("@/components/MemoEditor/services/uploadService", () => ({
  uploadService: { uploadFile: documentMocks.uploadFile },
}));

vi.mock("@/hooks/useDocumentQueries", () => ({
  useDocuments: () => ({ data: documentMocks.documents, isLoading: false, isError: false, refetch: vi.fn() }),
  useCreateDocument: () => ({ mutateAsync: documentMocks.createDocument }),
  useRetryDocument: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteDocument: () => ({ mutateAsync: vi.fn() }),
  getDocumentDeletePlan: documentMocks.getDeletePlan,
}));

vi.mock("@/hooks/useKnowledgeCardQueries", () => ({
  useKnowledgeCards: () => ({ data: [], isLoading: false, isError: false, refetch: vi.fn() }),
}));

describe("Knowledge page", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("en");
  });
  it("exposes only the four available modules as deep links", () => {
    render(
      <MemoryRouter>
        <Knowledge />
      </MemoryRouter>,
    );

    const navigation = screen.getByRole("navigation", { name: "Knowledge modules" });
    expect(within(navigation).getAllByRole("link")).toHaveLength(4);
    expect(within(navigation).getByRole("link", { name: /Document Inbox/ })).toHaveAttribute("href", "/knowledge/documents");
    expect(within(navigation).getByRole("link", { name: /Cards/ })).toHaveAttribute("href", "/knowledge/cards");
    expect(within(navigation).getByRole("link", { name: /Topics/ })).toHaveAttribute("href", "/knowledge/topics");
    expect(within(navigation).getByRole("link", { name: /Search/ })).toHaveAttribute("href", "/knowledge/search");
    expect(within(navigation).queryByRole("link", { name: /Review/ })).not.toBeInTheDocument();
    expect(within(navigation).getByText("Bring Word, Excel, PDF, Markdown, and text files into one processing queue.")).toBeInTheDocument();
    expect(screen.queryByText("Learn while you build")).not.toBeInTheDocument();
  });

  it("renders a simple Document Inbox with distinct outcomes and keyboard-reachable controls", () => {
    render(
      <MemoryRouter>
        <Knowledge moduleId="documents" />
      </MemoryRouter>,
    );

    expect(screen.getByRole("heading", { name: "Document Inbox" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Choose files" })).toBeInTheDocument();
    expect(screen.getByLabelText("Choose files")).toHaveAttribute("multiple");
    expect(screen.getAllByText("Ready")).toHaveLength(3);
    expect(screen.getByText("Parse failed")).toBeInTheDocument();
    expect(screen.getByText("Not supported yet")).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Download original" })).toHaveLength(4);
    expect(screen.getByRole("button", { name: "Read PDF" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to Knowledge" })).toHaveAttribute("href", "/knowledge");
    expect(screen.queryByText("Bring Word, Excel, PDF, Markdown, and text files into one processing queue.")).not.toBeInTheDocument();
    expect(screen.queryByText(/TXT and Markdown are readable now/)).not.toBeInTheDocument();
  });

  it("filters on a narrow-flow control without hiding the recovery action", () => {
    render(
      <MemoryRouter>
        <Knowledge moduleId="documents" />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Needs attention" }));
    expect(screen.queryByText("Learning notes")).not.toBeInTheDocument();
    expect(screen.getByText("Broken text")).toBeInTheDocument();
    expect(screen.getByText("Reference")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("opens the focused PDF reader with simple page controls", () => {
    render(
      <MemoryRouter>
        <Knowledge moduleId="documents" />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Read PDF" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Readable paper" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Next page" })).toBeDisabled();
    expect(within(screen.getByRole("dialog")).getByRole("link", { name: "Download original" })).toBeInTheDocument();
  });

  it("uploads multiple files independently and normalizes an empty Markdown media type", async () => {
    render(
      <MemoryRouter>
        <Knowledge moduleId="documents" />
      </MemoryRouter>,
    );

    const markdown = new File(["# Notes"], "notes.md");
    const text = new File(["Plain"], "plain.txt", { type: "text/plain" });
    fireEvent.change(screen.getByLabelText("Choose files"), { target: { files: [markdown, text] } });

    await waitFor(() => expect(documentMocks.uploadFile).toHaveBeenCalledTimes(2));
    expect(documentMocks.uploadFile.mock.calls[0][0].file.type).toBe("text/markdown");
    await waitFor(() => expect(documentMocks.createDocument).toHaveBeenCalledTimes(2));
  });

  it("states the exact deletion impact before confirmation", async () => {
    render(
      <MemoryRouter>
        <Knowledge moduleId="documents" />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getAllByRole("button", { name: "Remove" })[0]);
    expect(await screen.findByRole("heading", { name: "Remove notes.md?" })).toBeInTheDocument();
    expect(screen.getByText(/removes 1 derived content row.*2 parse attempt.*original uploaded file is kept/i)).toBeInTheDocument();
  });
});
