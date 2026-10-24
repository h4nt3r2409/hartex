"use client";

import * as React from "react";

import { setClientCookie } from "@/lib/cookie.client";
import { getLocalStorageValue, setLocalStorageValue } from "@/lib/local-storage.client";

import { isLocale, LOCALE_COOKIE, LOCALE_STORAGE_KEY, type Locale, resolveLocale } from "./config";

// The user's chosen display language is a purely frontend preference, so it is not stored
// on the server: it lives only in localStorage (and a cookie). Because it is not synced with
// the account, switching browsers or clearing site data reverts to the default language.
// The agent's output language is stored separately in the server settings.

function parseLocale(raw: string | null): Locale {
  return isLocale(raw) ? raw : resolveLocale();
}

// Set of subscribers within the same tab. The storage event fires only in "other" tabs, so
// after changing the language in the current tab, emit must notify that tab's subscribers
// for an immediate re-render.
const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  window.addEventListener("storage", listener);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", listener);
  };
}

function emit() {
  for (const listener of listeners) listener();
}

function getSnapshot(): Locale {
  return parseLocale(getLocalStorageValue(LOCALE_STORAGE_KEY));
}

// The server has no localStorage, so render the build default locale first; after hydration,
// getSnapshot corrects it to the user's chosen value.
function getServerSnapshot(): Locale {
  return resolveLocale();
}

// Hook that subscribes to the active display language. When the language changes,
// components using this hook re-render.
export function useActiveLocale(): Locale {
  return React.useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

// Change the display language. Writes to the cookie and localStorage, then notifies
// subscribers in the same tab. Persists for one year so the choice survives the next visit.
export function setActiveLocale(locale: Locale) {
  if (!isLocale(locale)) return;
  setLocalStorageValue(LOCALE_STORAGE_KEY, locale);
  setClientCookie(LOCALE_COOKIE, locale, 365);
  emit();
}
