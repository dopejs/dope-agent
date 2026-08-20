import { createRoot, hydrateRoot } from "react-dom/client";

import { App } from "./App";
import { readLanguagePreference } from "./language-preference";
import { localeForPath } from "./locales";
import "./styles.css";
import type { SitePayload } from "./types";

const legacy = location.hash.match(/^#\/?(docs(?:\/[^?#]+)?)(?:[?#].*)?$/u);
if (legacy !== null) {
  const path = legacy[1] === "docs" ? "docs/getting-started" : legacy[1];
  location.replace(`/${path.replace(/\/$/u, "")}/`);
}

async function payload(): Promise<{ readonly embedded: boolean; readonly value: SitePayload }> {
  const embedded = document.querySelector<HTMLScriptElement>("#kura-site-payload");
  if (embedded?.textContent != null) return { embedded: true, value: JSON.parse(embedded.textContent) as SitePayload };
  const endpoint = new URL("/__kura/site-page", location.origin);
  endpoint.searchParams.set("path", location.pathname);
  const response = await fetch(endpoint);
  if (!response.ok) throw new Error(`site page: ${String(response.status)}`);
  return { embedded: false, value: await response.json() as SitePayload };
}

const container = document.querySelector("#root");
if (container === null) throw new Error("Kura site root is missing");
void payload().then(({ embedded, value }) => {
  const localePath = readLanguagePreference(); const locale = localeForPath(localePath);
  document.documentElement.lang = locale.lang; document.documentElement.dir = locale.dir ?? "ltr";
  const app = <App payload={value} initialLocalePath={localePath} />;
  if (embedded && localePath === "en") hydrateRoot(container, app);
  else { container.replaceChildren(); createRoot(container).render(app); }
});
