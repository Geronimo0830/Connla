import { afterEach, describe, expect, it } from "vitest";
import i18n, { getInitialLocale } from "@/i18n";
import { applyLocaleEarly, getLocaleWithFallback } from "@/utils/i18n";

describe("Connla default language", () => {
  afterEach(() => {
    localStorage.clear();
  });

  it("starts in Simplified Chinese without an explicit Connla preference", () => {
    localStorage.setItem("memos-locale", "en");
    expect(getLocaleWithFallback()).toBe("zh-Hans");
  });

  it("keeps a language explicitly selected in Connla", () => {
    localStorage.setItem("connla-locale", "en");
    expect(getLocaleWithFallback()).toBe("en");
  });

  it("migrates a previous English preference to Chinese only once", () => {
    localStorage.setItem("connla-locale", "en");
    expect(getInitialLocale()).toBe("zh-Hans");
    expect(localStorage.getItem("connla-locale")).toBe("zh-Hans");

    localStorage.setItem("connla-locale", "en");
    expect(getInitialLocale()).toBe("en");
  });

  it("uses Chinese on startup even when the browser prefers English", async () => {
    localStorage.setItem("memos-locale", "en");
    applyLocaleEarly();
    await i18n.changeLanguage(getLocaleWithFallback());
    expect(i18n.language).toBe("zh-Hans");
    expect(document.documentElement.lang).toBe("zh-Hans");
  });
});
