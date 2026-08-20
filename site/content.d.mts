import type { PageSummary, SitePayload } from "./src/types";

export interface BuildPage extends PageSummary {
  readonly layout: "doc" | "home";
  readonly html: string;
  readonly tableOfContents: readonly { readonly id: string; readonly level: 2 | 3; readonly title: string }[];
  readonly lastUpdated: string;
}

export interface SiteContent {
  readonly pages: readonly BuildPage[];
  readonly searchIndex: readonly PageSummary[];
  payloadForPage(page: BuildPage): SitePayload;
  payloadForPath(pathname: string): SitePayload;
}

export function loadSiteContent(): Promise<SiteContent>;
