// Supported locales and the default. hartex defaults to English and adds Korean,
// Chinese, Spanish, and Japanese, for five languages the user can switch between in settings.
// Chinese ("zh") is also kept for comparison against the upstream original.
export const LOCALES = ["en", "ko", "zh", "es", "ja"] as const;
export type Locale = (typeof LOCALES)[number];

// The default display language is English. Korean, Chinese, Spanish, and Japanese are
// switchable in settings. The build can override this with NEXT_PUBLIC_LOCALE, and at runtime
// the value the user picks in the settings screen (cookie/localStorage) has the final say.
export const DEFAULT_LOCALE: Locale = "en";

// Each locale's native name, shown in the language switcher. A language's own name must be
// readable in that language regardless of the current UI language, so these are fixed
// labels, not translation targets.
export const LOCALE_LABELS: Record<Locale, string> = {
  en: "English",
  ko: "한국어",
  zh: "中文",
  es: "Español",
  ja: "日本語",
};

// Keys under which the user's chosen display language is stored. The UI reads the
// localStorage value. The same value is also written to a cookie, though nothing reads it
// yet (kept for when the server needs to determine the language).
export const LOCALE_COOKIE = "artex_locale";
export const LOCALE_STORAGE_KEY = "artex_locale";

// Guard that narrows an arbitrary string to a supported locale.
export function isLocale(raw: unknown): raw is Locale {
  return typeof raw === "string" && (LOCALES as readonly string[]).includes(raw);
}

// Determines the active locale at build time. It must stay compatible with static export
// (next.config's output: "export"), so it reads only an environment variable rather than
// dynamic APIs such as cookies() or headers(). If NEXT_PUBLIC_LOCALE is empty or outside the
// supported list, it falls back to the default. This value is used only in the server
// prerender's initial HTML; the actual user display language is overridden client-side by a cookie.
export function resolveLocale(): Locale {
  const raw = process.env.NEXT_PUBLIC_LOCALE;
  return isLocale(raw) ? raw : DEFAULT_LOCALE;
}
