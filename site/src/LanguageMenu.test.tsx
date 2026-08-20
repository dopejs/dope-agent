import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { LanguageMenu } from "./LanguageMenu";
import { SITE_LOCALES } from "./locales";

describe("LanguageMenu", () => {
  it("renders the dopejs-style language trigger without a native select", () => {
    const locale = SITE_LOCALES.find((candidate) => candidate.lang === "zh-Hans");
    expect(locale).toBeDefined();

    const html = renderToStaticMarkup(<LanguageMenu locale={locale!} onChange={() => undefined} />);

    expect(html).toContain('aria-expanded="false"');
    expect(html).toContain('aria-label="语言"');
    expect(html).toContain("简体中文");
    expect(html).toContain("<svg");
    expect(html).not.toContain("<select");
  });
});
