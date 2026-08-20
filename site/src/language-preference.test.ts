import { describe, expect, it } from "vitest";

import { languageCookie, matchSupportedLanguage } from "./language-preference";

describe("shared language preference", () => {
  it("maps Chinese variants", () => {
    expect(matchSupportedLanguage("zh-CN")).toBe("");
    expect(matchSupportedLanguage("zh-TW")).toBe("zh-Hant");
  });

  it("matches regional tags", () => {
    expect(matchSupportedLanguage("en-US")).toBe("en");
    expect(matchSupportedLanguage("ja-JP")).toBe("ja");
  });

  it("shares cookies only on dopejs.com", () => {
    expect(languageCookie("ja", "agent.kurajs.com", true)).toBe("dopejs_locale=ja; Path=/; Max-Age=31536000; SameSite=Lax; Secure");
    expect(languageCookie("ja", "kura.dopejs.com", true)).toBe("dopejs_locale=ja; Path=/; Max-Age=31536000; SameSite=Lax; Domain=dopejs.com; Secure");
  });
});
