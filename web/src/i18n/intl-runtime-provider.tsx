"use client";

import type { ReactNode } from "react";

import { NextIntlClientProvider } from "next-intl";

import enMessages from "../../messages/en.json";
import { LOCALE } from "./config";

// Time zone of the runtime environment (browser or prerender server).
const TIME_ZONE = Intl.DateTimeFormat().resolvedOptions().timeZone;

// Supplies the single English catalog. Unlike the root layout's provider, this one does not
// inherit server request config, so timeZone is passed explicitly. Without it, next-intl logs
// an ENVIRONMENT_FALLBACK error during prerender.
export function IntlRuntimeProvider({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <NextIntlClientProvider locale={LOCALE} messages={enMessages} timeZone={TIME_ZONE}>
      {children}
    </NextIntlClientProvider>
  );
}
