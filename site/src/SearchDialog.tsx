import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import type { SiteLocale } from "./locales";
import type { PageSummary } from "./types";

export function SearchDialog({ open, locale, onClose }: { readonly open: boolean; readonly locale: SiteLocale; onClose(): void }): ReactNode {
  const [query, setQuery] = useState("");
  const [records, setRecords] = useState<readonly PageSummary[]>([]);
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (!open || records.length > 0) return;
    const controller = new AbortController();
    void fetch("/__kura/search-index.json", { signal: controller.signal }).then((response) => {
      if (!response.ok) throw new Error(`search index: ${String(response.status)}`);
      return response.json() as Promise<readonly PageSummary[]>;
    }).then(setRecords).catch(() => undefined);
    return () => controller.abort();
  }, [open, records.length]);
  useEffect(() => {
    if (!open) return;
    input.current?.focus();
    const handler = (event: KeyboardEvent): void => { if (event.key === "Escape") onClose(); };
    window.addEventListener("keydown", handler); return () => window.removeEventListener("keydown", handler);
  }, [onClose, open]);
  const results = useMemo(() => {
    const terms = query.trim().toLocaleLowerCase().split(/\s+/u).filter(Boolean);
    if (terms.length === 0) return [];
    return records.map((record) => {
      const title = record.title.toLocaleLowerCase(); const body = `${record.headings.join(" ")} ${record.text}`.toLocaleLowerCase();
      if (terms.some((term) => !title.includes(term) && !body.includes(term))) return { record, score: -1 };
      return { record, score: terms.reduce((value, term) => value + (title.includes(term) ? 10 : 1), 0) };
    }).filter(({ score }) => score >= 0).sort((a, b) => b.score - a.score).slice(0, 10);
  }, [query, records]);
  if (!open) return null;
  return <div className="search-backdrop" role="presentation" onMouseDown={onClose}><section className="search-dialog" role="dialog" aria-modal="true" aria-label={locale.ui.search} onMouseDown={(event) => event.stopPropagation()}><label className="search-field"><span aria-hidden="true">⌕</span><input ref={input} value={query} placeholder={locale.ui.searchPlaceholder} onChange={(event) => setQuery(event.currentTarget.value)} /><button type="button" onClick={onClose}>Esc</button></label><div className="search-results">{query !== "" && results.length === 0 && <p>{locale.ui.noResults}</p>}{results.map(({ record }) => <a key={record.route} href={record.href}><strong>{record.title}</strong><span>{record.description}</span></a>)}</div></section></div>;
}
