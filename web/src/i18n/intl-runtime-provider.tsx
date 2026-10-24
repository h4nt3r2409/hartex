"use client";

import type { ReactNode } from "react";
import * as React from "react";

import { NextIntlClientProvider } from "next-intl";

import enMessages from "../../messages/en.json";
import esMessages from "../../messages/es.json";
import jaMessages from "../../messages/ja.json";
import koMessages from "../../messages/ko.json";
import zhMessages from "../../messages/zh.json";
import type { Locale } from "./config";
import { useActiveLocale } from "./locale-store";

// Statically bundle the messages for all five languages. That way, when the user changes
// the language in the settings screen, every string re-renders in the selected language
// on the spot, with no rebuild, page reload, or async loading (also compatible with
// static export output: "export").
const MESSAGES: Record<Locale, Record<string, unknown>> = {
  en: enMessages as Record<string, unknown>,
  ko: koMessages as Record<string, unknown>,
  zh: zhMessages as Record<string, unknown>,
  es: esMessages as Record<string, unknown>,
  ja: jaMessages as Record<string, unknown>,
};

// Time zone of the runtime environment (browser or prerender server).
const TIME_ZONE = Intl.DateTimeFormat().resolvedOptions().timeZone;

// Provider responsible for runtime language switching. It replaces the root layout's
// NextIntlClientProvider and supplies the messages selected for the user's active locale
// (cookie/localStorage).
export function IntlRuntimeProvider({ children }: Readonly<{ children: ReactNode }>) {
  const locale = useActiveLocale();

  // Keep <html lang> in sync when the language changes. This overrides the initial lang
  // seeded by the root layout on the server with the client-side choice, so screen readers
  // and browser translation operate in the correct language.
  React.useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  // Unlike the root layout's provider, this one does not inherit server request config, so
  // timeZone is passed explicitly. Without it, next-intl logs an ENVIRONMENT_FALLBACK error
  // during prerender.
  return (
    <NextIntlClientProvider locale={locale} messages={MESSAGES[locale]} timeZone={TIME_ZONE}>
      {children}
    </NextIntlClientProvider>
  );
}
