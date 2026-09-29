import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import MemosLogo from "@/components/MemosLogo";

const instance = {
  generalSetting: {
    customProfile: undefined as { title: string; logoUrl: string } | undefined,
  },
};

vi.mock("@/contexts/InstanceContext", () => ({
  useInstance: () => instance,
}));

describe("Connla sidebar identity", () => {
  beforeEach(() => {
    instance.generalSetting.customProfile = undefined;
  });

  it("shows the Connla icon when no custom profile exists", () => {
    const { container } = render(<MemosLogo compact />);

    expect(screen.getByText("Connla")).toBeInTheDocument();
    expect(container.querySelector("img")).toHaveAttribute("src", "/full-logo.webp?v=connla-1");
  });

  it("keeps an explicitly customized icon", () => {
    instance.generalSetting.customProfile = { title: "My Notes", logoUrl: "/custom-logo.png" };
    const { container } = render(<MemosLogo compact />);

    expect(screen.getByText("My Notes")).toBeInTheDocument();
    expect(container.querySelector("img")).toHaveAttribute("src", "/custom-logo.png");
  });
});
