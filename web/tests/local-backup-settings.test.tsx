import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import LocalBackupSection from "@/components/Settings/LocalBackupSection";
import { getVisibleSettingSections } from "@/components/Settings/settingSections";
import i18n from "@/i18n";

const originalChrome = Object.getOwnPropertyDescriptor(window, "chrome");

afterEach(() => {
  if (originalChrome) Object.defineProperty(window, "chrome", originalChrome);
  else Reflect.deleteProperty(window, "chrome");
});

beforeEach(async () => {
  await i18n.changeLanguage("zh-Hans");
});

describe("desktop backup settings", () => {
  it("keeps whole-library maintenance out of the browser and non-admin settings", () => {
    expect(getVisibleSettingSections(true).some((section) => section.key === "local-backup")).toBe(false);
    expect(getVisibleSettingSections(false).some((section) => section.key === "local-backup")).toBe(false);
  });

  it("shows two actions only in the desktop host and sends no filesystem path from JavaScript", async () => {
    const listeners = new Set<(event: MessageEvent) => void>();
    const postMessage = vi.fn();
    Object.defineProperty(window, "chrome", {
      configurable: true,
      value: {
        webview: {
          postMessage,
          addEventListener: (_type: string, listener: (event: MessageEvent) => void) => listeners.add(listener),
          removeEventListener: (_type: string, listener: (event: MessageEvent) => void) => listeners.delete(listener),
        },
      },
    });
    expect(getVisibleSettingSections(true).some((section) => section.key === "local-backup")).toBe(true);
    expect(getVisibleSettingSections(false).some((section) => section.key === "local-backup")).toBe(false);

    render(<LocalBackupSection />);
    fireEvent.click(screen.getByRole("button", { name: "创建备份" }));
    const request = postMessage.mock.calls[0]?.[0] as Record<string, unknown>;
    expect(request).toMatchObject({ type: "connla:maintenance", action: "backup" });
    expect(Object.keys(request).sort()).toEqual(["action", "id", "type"]);
    for (const listener of listeners) {
      listener(
        new MessageEvent("message", {
          data: { type: "connla:maintenance-result", id: request.id, status: "success", message: "已验证备份" },
        }),
      );
    }
    expect(await screen.findByRole("status")).toHaveTextContent("已验证备份");
  });
});
