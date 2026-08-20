export type PageLayout = "doc" | "home";

export interface TableOfContentsItem {
  readonly id: string;
  readonly level: 2 | 3;
  readonly title: string;
}

export interface SitePage {
  readonly route: string;
  readonly href: string;
  readonly title: string;
  readonly description: string;
  readonly layout: PageLayout;
  readonly html: string;
  readonly tableOfContents: readonly TableOfContentsItem[];
  readonly lastUpdated: string;
}

export interface PageSummary {
  readonly route: string;
  readonly href: string;
  readonly title: string;
  readonly description: string;
  readonly headings: readonly string[];
  readonly text: string;
}

export interface PageLink {
  readonly href: string;
  readonly title: string;
}

export interface SitePayload {
  readonly page: SitePage;
  readonly previous?: PageLink;
  readonly next?: PageLink;
}
