import { expect, test } from "vitest";

import { LOCALE } from "./config";

test("the UI has exactly one locale", () => {
  expect(LOCALE).toBe("en");
});
