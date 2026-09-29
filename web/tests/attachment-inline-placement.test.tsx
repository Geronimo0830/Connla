import { create } from "@bufbuild/protobuf";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeAll, describe, expect, it, vi } from "vitest";
import AttachmentListEditor from "@/components/MemoMetadata/Attachment/AttachmentListEditor";
import { AttachmentSchema } from "@/types/proto/api/v1/attachment_service_pb";

const image = create(AttachmentSchema, {
  name: "attachments/photo",
  filename: "photo.png",
  type: "image/png",
  size: 128n,
});

beforeAll(() => {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: () => ({
      matches: false,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    }),
  });
});

describe("attachment inline placement actions", () => {
  it("offers insertion for an attachment-only image", () => {
    const onInsertAttachments = vi.fn();
    render(
      <AttachmentListEditor
        attachments={[image]}
        inlineAttachmentUIDs={new Set()}
        onAttachmentsChange={vi.fn()}
        onInsertAttachments={onInsertAttachments}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "插入图片" }));
    expect(onInsertAttachments).toHaveBeenCalledWith([image]);
    expect(screen.getByRole("button", { name: "移除附件" })).toBeInTheDocument();
  });

  it("hides an inline image and restores it after its Markdown reference is removed", () => {
    const props = {
      attachments: [image],
      onAttachmentsChange: vi.fn(),
    };
    const { rerender } = render(<AttachmentListEditor {...props} inlineAttachmentUIDs={new Set(["photo"])} />);

    expect(screen.queryByText("附件")).not.toBeInTheDocument();
    expect(screen.queryByText("photo.png")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "移除附件" })).not.toBeInTheDocument();

    rerender(<AttachmentListEditor {...props} inlineAttachmentUIDs={new Set()} />);

    expect(screen.getByText(/附件/)).toBeInTheDocument();
    expect(screen.getByText("photo.png")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "移除附件" })).toBeInTheDocument();
  });

  it("preserves hidden inline attachments when visible attachments are reordered", () => {
    const second = create(AttachmentSchema, { ...image, name: "attachments/second", filename: "second.png" });
    const third = create(AttachmentSchema, { ...image, name: "attachments/third", filename: "third.png" });
    const onAttachmentsChange = vi.fn();
    render(
      <AttachmentListEditor
        attachments={[image, second, third]}
        inlineAttachmentUIDs={new Set(["photo"])}
        onAttachmentsChange={onAttachmentsChange}
      />,
    );

    fireEvent.click(screen.getAllByRole("button", { name: "下移附件" })[0]!);
    expect(onAttachmentsChange).toHaveBeenCalledWith([image, third, second]);
  });

  it("does not offer inline placement for external images", () => {
    render(
      <AttachmentListEditor
        attachments={[create(AttachmentSchema, { ...image, externalLink: "https://example.com/photo.png" })]}
        onAttachmentsChange={vi.fn()}
      />,
    );

    expect(screen.queryByRole("button", { name: "插入图片" })).not.toBeInTheDocument();
  });

  it("shows local inline-upload status and prevents removal while uploading", () => {
    const localImage = {
      file: new File(["image"], "local.png", { type: "image/png" }),
      previewUrl: "blob:local",
      origin: "upload" as const,
    };
    render(
      <AttachmentListEditor
        attachments={[]}
        localFiles={[localImage]}
        uploadingLocalFileURLs={new Set([localImage.previewUrl])}
        onLocalFilesChange={vi.fn()}
        onInsertLocalFiles={vi.fn()}
        placementActionsDisabled
      />,
    );

    expect(screen.getByText("正在上传到正文")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "插入图片" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "移除附件" })).not.toBeInTheDocument();
  });
});
