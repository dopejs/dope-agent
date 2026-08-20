import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");

describe("published brand assets", () => {
  for (const name of ["kura-mark.svg", "kura-mark-inverse.svg"]) {
    it(`${name} matches the authoritative brand asset`, async () => {
      const canonical = await readFile(path.join(repositoryRoot, "brand", name), "utf8");
      const [site, web] = await Promise.all([
        readFile(path.join(repositoryRoot, "site", "public", name), "utf8"),
        readFile(path.join(repositoryRoot, "web", "public", name), "utf8"),
      ]);
      expect(site).toBe(canonical);
      expect(web).toBe(canonical);
    });
  }
});
