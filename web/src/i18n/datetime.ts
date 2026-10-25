// BCP47 locale tag used for formatting dates and times. The product is English-only, so this
// is a fixed constant and every locale-sensitive formatting site uses it.
export function getDateLocale(): string {
  return "en-US";
}
