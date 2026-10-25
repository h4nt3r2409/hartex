import { expect, test } from "vitest";

import enMessages from "../../messages/en.json";
import { LOCALE } from "./config";

// Han, Hangul, Hiragana, and Katakana. Property escapes keep this file ASCII-only
// (the source-language guard scans it) and match the character classes used by
// scripts/check-source-language.py and scripts/check-web-cjk.py.
const CJK_TEXT = /[\p{Script=Han}\p{Script=Hangul}\p{Script=Hiragana}\p{Script=Katakana}]/u;

function collectStrings(value: unknown, path: string, out: Array<[path: string, text: string]>): void {
  if (typeof value === "string") {
    out.push([path, value]);
    return;
  }
  if (Array.isArray(value)) {
    value.forEach((item, index) => {
      collectStrings(item, `${path}[${index}]`, out);
    });
    return;
  }
  if (value !== null && typeof value === "object") {
    for (const [key, child] of Object.entries(value)) {
      collectStrings(child, path === "" ? key : `${path}.${key}`, out);
    }
  }
}

test("the UI has exactly one locale", () => {
  expect(LOCALE).toBe("en");
});

test("the English message catalog contains no CJK text", () => {
  const strings: Array<[path: string, text: string]> = [];
  collectStrings(enMessages, "", strings);
  const offenders = strings.filter(([, text]) => CJK_TEXT.test(text)).map(([path, text]) => `${path}: ${text}`);
  expect(offenders).toEqual([]);
});
