import { execFile } from "node:child_process";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { promisify } from "node:util";

import { loadSiteContent } from "../site/content.mjs";

const run = promisify(execFile);
const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const siteRoot = path.join(repositoryRoot, "site");
const output = path.join(siteRoot, "dist");
const serverOutput = path.join(siteRoot, ".ssr");

function escapeAttribute(value) {
  return value.replaceAll("&", "&amp;").replaceAll('"', "&quot;").replaceAll("<", "&lt;");
}
function embeddedJson(value) { return JSON.stringify(value).replaceAll("<", "\\u003c").replaceAll("-->", "--\\u003e"); }
function outputPathForRoute(route) { return route === "/" ? "index.html" : `${route.slice(1)}/index.html`; }

await rm(output, { recursive: true, force: true });
await rm(serverOutput, { recursive: true, force: true });
try {
  await run("pnpm", ["--dir", "site", "exec", "vite", "build"], { cwd: repositoryRoot });
  await run("pnpm", ["--dir", "site", "exec", "vite", "build", "--ssr", "src/ssr.tsx", "--outDir", ".ssr", "--emptyOutDir"], { cwd: repositoryRoot });
  const [{ render }, template, content] = await Promise.all([
    import(pathToFileURL(path.join(serverOutput, "ssr.js")).href),
    readFile(path.join(output, "index.html"), "utf8"),
    loadSiteContent(),
  ]);
  for (const page of content.pages) {
    const payload = content.payloadForPage(page);
    const rendered = render(payload);
    const title = page.layout === "home" ? "Kura — A Personal Agent OS" : `${page.title} | Kura`;
    const html = template
      .replace("<title>Kura — A Personal Agent OS</title>", `<title>${escapeAttribute(title)}</title>`)
      .replace(/<meta name="description" content="[^"]+" \/>/u, `<meta name="description" content="${escapeAttribute(page.description)}" />`)
      .replace('<div id="root"></div>', `<div id="root">${rendered}</div><script id="kura-site-payload" type="application/json">${embeddedJson(payload)}</script>`);
    const target = path.join(output, outputPathForRoute(page.route));
    await mkdir(path.dirname(target), { recursive: true });
    await writeFile(target, html);
  }
  const searchDirectory = path.join(output, "__kura");
  await mkdir(searchDirectory, { recursive: true });
  await writeFile(path.join(searchDirectory, "search-index.json"), JSON.stringify(content.searchIndex));
  const docsRoot = path.join(output, "docs", "index.html");
  await mkdir(path.dirname(docsRoot), { recursive: true });
  await writeFile(
    docsRoot,
    '<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="robots" content="noindex"><link rel="canonical" href="https://kura.dopejs.com/docs/getting-started/"><meta http-equiv="refresh" content="0;url=/docs/getting-started/"><title>Kura documentation</title></head><body><script>location.replace("/docs/getting-started/"+location.search+location.hash)</script><a href="/docs/getting-started/">Continue to Kura documentation</a></body></html>',
  );
  process.stdout.write(`Kura React site built: ${String(content.pages.length)} static pages, 1 redirect\n`);
} finally {
  await rm(serverOutput, { recursive: true, force: true });
}
