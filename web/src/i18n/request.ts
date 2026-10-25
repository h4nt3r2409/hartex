import { getRequestConfig } from "next-intl/server";

import enMessages from "../../messages/en.json";
import { LOCALE } from "./config";

// next-intl request config for the single English catalog. This setup does not use i18n path
// routing (segments or middleware), so the locale is fixed here. It never references
// requestLocale, so it does not trigger dynamic rendering.
export default getRequestConfig(async () => ({
  locale: LOCALE,
  messages: enMessages,
}));
