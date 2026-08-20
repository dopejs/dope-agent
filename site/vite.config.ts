import react from "@vitejs/plugin-react";
import type { Plugin } from "vite";
import { defineConfig } from "vite";

interface DevelopmentContent {
  payloadForPath(pathname: string): unknown;
  readonly searchIndex: unknown;
}

function developmentContent(): Plugin {
  let contentPromise: Promise<DevelopmentContent> | undefined;
  const content = (): Promise<DevelopmentContent> => {
    contentPromise ??= import("./content.mjs").then(async ({ loadSiteContent }) => await loadSiteContent() as DevelopmentContent);
    return contentPromise;
  };
  return {
    name: "kura-site-content",
    configureServer(server) {
      server.middlewares.use((request, response, next) => {
        const url = new URL(request.url ?? "/", "http://kura.local");
        const send = (value: unknown): void => {
          response.statusCode = 200;
          response.setHeader("Content-Type", "application/json; charset=utf-8");
          response.setHeader("Cache-Control", "no-store");
          response.end(JSON.stringify(value));
        };
        if (url.pathname === "/__kura/site-page") {
          void content().then((site) => send(site.payloadForPath(url.searchParams.get("path") ?? "/"))).catch(next);
          return;
        }
        if (url.pathname === "/__kura/search-index.json") {
          void content().then((site) => send(site.searchIndex)).catch(next);
          return;
        }
        next();
      });
    },
  };
}

export default defineConfig({
  root: new URL(".", import.meta.url).pathname,
  base: "/",
  plugins: [react(), developmentContent()],
  publicDir: "public",
  build: { outDir: "dist", emptyOutDir: true, sourcemap: true },
});
