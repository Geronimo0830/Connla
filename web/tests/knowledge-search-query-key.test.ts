import { describe, expect, it } from "vitest";
import { searchKeys, toSearchDateBound } from "@/hooks/useKnowledgeSearch";

describe("knowledge search query key", () => {
  it("is serializable when date filters use protobuf int64 values", () => {
    const key = searchKeys.query("训练", "", "document", ".docx", 1788192000n, 1790726400n);
    expect(() => JSON.stringify(key)).not.toThrow();
    expect(key.at(-2)).toBe("1788192000");
  });

  it("tolerates incomplete browser date input without BigInt errors", () => {
    expect(toSearchDateBound("2026-09-", false)).toBe(0n);
    expect(toSearchDateBound("", true)).toBe(0n);
    expect(toSearchDateBound("2026-09-29", true)).toBe(BigInt(Math.floor(new Date("2026-09-30T00:00:00").getTime() / 1000)));
  });
});
