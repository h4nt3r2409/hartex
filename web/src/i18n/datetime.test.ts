import { expect, test } from "vitest";

import { getDateLocale } from "./datetime";

test("date formatting is pinned to English", () => {
  expect(getDateLocale()).toBe("en-US");
});
