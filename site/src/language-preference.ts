import { SITE_LOCALES } from "./locales";

export const LANGUAGE_STORAGE_KEY = "dopejs.locale";
export const LANGUAGE_COOKIE_KEY = "dopejs_locale";

export function matchSupportedLanguage(value: string | null | undefined): string | undefined {
  if (value == null) return undefined;
  const normalized = value.trim().toLocaleLowerCase();
  if (["zh-hans", "zh-cn", "zh-sg"].includes(normalized)) return "";
  if (["zh-hant", "zh-tw", "zh-hk", "zh-mo"].includes(normalized)) return "zh-Hant";
  const exact = SITE_LOCALES.find((locale) =>
    locale.path.toLocaleLowerCase() === normalized || locale.lang.toLocaleLowerCase() === normalized,
  );
  if (exact !== undefined) return exact.path;
  const language = normalized.split("-")[0];
  return SITE_LOCALES.find((locale) => locale.lang.toLocaleLowerCase().split("-")[0] === language)?.path;
}

export function languageCookie(value: string, hostname: string, secure: boolean): string {
  const domain = hostname === "dopejs.com" || hostname.endsWith(".dopejs.com") ? "; Domain=dopejs.com" : "";
  return `${LANGUAGE_COOKIE_KEY}=${encodeURIComponent(value)}; Path=/; Max-Age=31536000; SameSite=Lax${domain}${secure ? "; Secure" : ""}`;
}

function cookiePreference(): string | undefined {
  const prefix = `${LANGUAGE_COOKIE_KEY}=`;
  for (const part of document.cookie.split(";")) {
    const item = part.trim();
    if (item.startsWith(prefix)) return matchSupportedLanguage(decodeURIComponent(item.slice(prefix.length)));
  }
  return undefined;
}

export function readLanguagePreference(): string {
  try {
    const local = matchSupportedLanguage(localStorage.getItem(LANGUAGE_STORAGE_KEY));
    if (local !== undefined) return local;
  } catch {}
  const cookie = cookiePreference();
  if (cookie !== undefined) return cookie;
  for (const language of navigator.languages.length === 0 ? [navigator.language] : navigator.languages) {
    const resolved = matchSupportedLanguage(language);
    if (resolved !== undefined) return resolved;
  }
  return "en";
}

export function writeLanguagePreference(path: string): void {
  const locale = SITE_LOCALES.find((candidate) => candidate.path === path) ?? SITE_LOCALES[0];
  try { localStorage.setItem(LANGUAGE_STORAGE_KEY, locale.lang); } catch {}
  document.cookie = languageCookie(locale.lang, location.hostname, location.protocol === "https:");
}
