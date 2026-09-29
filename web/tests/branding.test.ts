import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const publicPath = (name: string) => resolve(import.meta.dirname, "../public", name);

describe("Connla branding", () => {
  it("uses Connla for the browser title and installable app", () => {
    const html = readFileSync(resolve(import.meta.dirname, "../index.html"), "utf8");
    const manifest = JSON.parse(readFileSync(publicPath("site.webmanifest"), "utf8"));

    expect(html).toContain("<title>Connla</title>");
    expect(html).toContain('href="/logo.webp?v=connla-1"');
    expect(manifest.name).toBe("Connla");
    expect(manifest.short_name).toBe("Connla");
  });

  it.each([
    ["apple-touch-icon.png", 180],
    ["android-chrome-192x192.png", 192],
    ["android-chrome-512x512.png", 512],
  ])("exports the %s icon at %i square pixels", (filename, size) => {
    const png = readFileSync(publicPath(filename));
    expect(png.subarray(0, 8).toString("hex")).toBe("89504e470d0a1a0a");
    expect(png.readUInt32BE(16)).toBe(size);
    expect(png.readUInt32BE(20)).toBe(size);
  });

  it.each(["logo.webp", "full-logo.webp"])("exports the %s web icon", (filename) => {
    const webp = readFileSync(publicPath(filename));
    expect(webp.toString("ascii", 0, 4)).toBe("RIFF");
    expect(webp.toString("ascii", 8, 12)).toBe("WEBP");
  });
});
