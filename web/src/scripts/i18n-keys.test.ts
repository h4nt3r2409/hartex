import { describe, expect, it } from "vitest";

import { diffKeys, flattenKeys, isInSync } from "./i18n-keys";
import { readFileSync } from "node:fs";
import path from "node:path";

const MESSAGES_DIR = path.resolve(process.cwd(), "messages");

function readKeys(locale: string): Set<string> {
  const tree = JSON.parse(readFileSync(path.join(MESSAGES_DIR, `${locale}.json`), "utf8")) as unknown;
  return flattenKeys(tree);
}

describe("i18n key helpers", () => {
  // The product has a single English catalog now; the five-locale parity guard this file
  // used to exercise is removed with the other catalogs in Task 6.
  it("reads the single English catalog", () => {
    const baseline = readKeys("en");
    expect(baseline.size).toBeGreaterThan(0);
  });

  it("detects missing and extra keys with the same helper the CLI uses", () => {
    const baseline = new Set(["a", "a.b", "a.c"]);
    const target = new Set(["a", "a.b", "a.d"]);
    const diff = diffKeys(baseline, target);
    expect(diff.missing).toEqual(["a.c"]);
    expect(diff.extra).toEqual(["a.d"]);
    expect(isInSync(diff)).toBe(false);
  });

  it("flattens nested trees and keeps array and primitive leaves", () => {
    expect(flattenKeys({ a: { b: "x" }, c: ["y"] })).toEqual(new Set(["a.b", "c"]));
  });
});
