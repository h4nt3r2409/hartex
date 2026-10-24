/**
 * i18n locale key consistency helper.
 *
 * Compares each locale's "flattened key set" against ko.json as the baseline and finds
 * both missing and extra keys. The CLI (check-i18n-keys.ts) and the Vitest tests share
 * this pure logic, so the validation rule is defined in exactly one place.
 */

export interface KeyDiff {
  missing: string[]; // keys present in baseline but absent from target
  extra: string[]; // keys absent from baseline but present in target
}

/** Flatten a nested JSON object into a key set in "a.b.c" form. Arrays and primitives are treated as leaves. */
export function flattenKeys(tree: unknown, prefix = ""): Set<string> {
  const out = new Set<string>();
  if (tree === null || typeof tree !== "object" || Array.isArray(tree)) {
    if (prefix) out.add(prefix);
    return out;
  }
  for (const [key, value] of Object.entries(tree as Record<string, unknown>)) {
    const path = prefix ? `${prefix}.${key}` : key;
    for (const leaf of flattenKeys(value, path)) out.add(leaf);
  }
  return out;
}

/** Return target's missing and extra keys relative to baseline as sorted arrays. */
export function diffKeys(baseline: Set<string>, target: Set<string>): KeyDiff {
  const missing = [...baseline].filter((k) => !target.has(k)).sort();
  const extra = [...target].filter((k) => !baseline.has(k)).sort();
  return { missing, extra };
}

/** True when both missing and extra are empty. */
export function isInSync(diff: KeyDiff): boolean {
  return diff.missing.length === 0 && diff.extra.length === 0;
}
