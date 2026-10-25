// BCP47 locale tag used for formatting dates and times. The product is English-only, so this
// is a constant. Task 5 pins the remaining locale-sensitive formatting sites to it.
export function getDateLocale(): string {
  return "en-US";
}
