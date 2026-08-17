//! dope-context — the default context-management policy engine.
//!
//! This crate backs the `context` builtin plugin, which attaches at
//! `chat/pre-dispatch` and injects the tenant's memory bootstrap (Ready L3
//! persona first, then Ready L2 scenarios, newest first) into the dispatch's
//! system frame — **deterministically, under an explicit budget, with the
//! citation inline**. Design root: TencentDB-Agent-Memory (L3/L2 bootstrap
//! under budgets; L1 atoms are never bulk-injected — they arrive via
//! drill-down or retrieval).
//!
//! Everything included or excluded is recorded in an [`AssemblyRecord`]:
//! nothing enters or misses the context silently. The record rides a
//! `context.assembled` event so an engineer can reconstruct exactly what
//! memory the model saw for any dispatch.
//!
//! Because this runs on the hook waterfall, other plugins modify the result
//! by design: the `session-strategy` plugin shapes the window after
//! injection (memory bootstrap messages are system-frame and survive
//! elision), and any later builtin or external hook may rewrite or veto.

use serde::{Deserialize, Serialize};

/// One candidate memory asset for bootstrap injection (a structural subset
/// of the memory plane's asset; this crate stays decoupled on purpose).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct BootstrapAsset {
    pub asset_id: String,
    /// `l3` or `l2` (already filtered/ordered by the caller).
    pub layer: String,
    pub title: String,
    pub content: String,
}

/// Plugin configuration (the `config` object of the `context` entry in
/// `plugins.json`). Zero values fall back to defaults.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct ContextConfig {
    /// Total character budget for injected bootstrap content (L3/L2).
    pub memory_budget_chars: usize,
    /// Character budget for query-time recall of L1 atoms.
    pub retrieval_budget_chars: usize,
}

/// Default memory bootstrap budget (chars of asset content).
pub const DEFAULT_MEMORY_BUDGET_CHARS: usize = 4000;
/// Default retrieval budget (chars of recalled L1 content).
pub const DEFAULT_RETRIEVAL_BUDGET_CHARS: usize = 2000;
/// Maximum scored candidates considered per retrieval pass.
pub const RETRIEVAL_MAX_CANDIDATES: usize = 8;

impl ContextConfig {
    #[must_use]
    pub fn budget(&self) -> usize {
        if self.memory_budget_chars > 0 {
            self.memory_budget_chars
        } else {
            DEFAULT_MEMORY_BUDGET_CHARS
        }
    }

    #[must_use]
    pub fn retrieval_budget(&self) -> usize {
        if self.retrieval_budget_chars > 0 {
            self.retrieval_budget_chars
        } else {
            DEFAULT_RETRIEVAL_BUDGET_CHARS
        }
    }
}

/// A message to inject (role is always `system` for bootstrap content).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ContextMessage {
    pub role: String,
    pub content: String,
}

/// Default item source in the assembly record (bootstrap injection).
fn default_source() -> String {
    "bootstrap".to_string()
}

/// One included asset in the assembly record.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct IncludedItem {
    pub asset_id: String,
    pub layer: String,
    pub chars: usize,
    /// `bootstrap` | `retrieval`.
    #[serde(default = "default_source")]
    pub source: String,
}

/// One excluded asset and why.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ExcludedItem {
    pub asset_id: String,
    pub layer: String,
    /// `over_budget` | `empty_content` | `visibility`.
    pub reason: String,
    /// `bootstrap` | `retrieval`.
    #[serde(default = "default_source")]
    pub source: String,
}

/// The full decision record of one assembly pass.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AssemblyRecord {
    pub included: Vec<IncludedItem>,
    pub excluded: Vec<ExcludedItem>,
    pub budget_chars: usize,
    pub used_chars: usize,
    #[serde(default)]
    pub retrieval_budget_chars: usize,
    #[serde(default)]
    pub retrieval_used_chars: usize,
}

impl AssemblyRecord {
    /// True when the pass neither included nor excluded anything (no memory
    /// to speak of — callers skip the event in that case).
    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.included.is_empty() && self.excluded.is_empty()
    }
}

/// Renders one asset as its injected system message: the citation (layer +
/// asset id) stays inline so recalled memory is evidence, never bare text.
#[must_use]
pub fn render_bootstrap_message(asset: &BootstrapAsset) -> String {
    let title = asset.title.trim();
    if title.is_empty() {
        format!("Memory[{} {}]: {}", asset.layer, asset.asset_id, asset.content.trim())
    } else {
        format!(
            "Memory[{} {}] {}: {}",
            asset.layer, asset.asset_id, title, asset.content.trim()
        )
    }
}

/// Assembles the bootstrap injection: walks `assets` in the caller's order
/// (L3 first, then L2 newest-first), including while the content budget
/// holds. Deterministic: same input → same output. Assets with empty
/// content are excluded (`empty_content`), assets past the budget are
/// excluded (`over_budget`) — the walk continues so a small L2 can still
/// fit after a large one was excluded.
#[must_use]
pub fn assemble(
    assets: &[BootstrapAsset],
    budget_chars: usize,
) -> (Vec<ContextMessage>, AssemblyRecord) {
    let mut messages = Vec::new();
    let mut record = AssemblyRecord {
        budget_chars,
        ..AssemblyRecord::default()
    };
    for asset in assets {
        let content_len = asset.content.trim().len();
        if content_len == 0 {
            record.excluded.push(ExcludedItem {
                asset_id: asset.asset_id.clone(),
                layer: asset.layer.clone(),
                reason: "empty_content".to_string(),
                source: default_source(),
            });
            continue;
        }
        if record.used_chars + content_len > budget_chars {
            record.excluded.push(ExcludedItem {
                asset_id: asset.asset_id.clone(),
                layer: asset.layer.clone(),
                reason: "over_budget".to_string(),
                source: default_source(),
            });
            continue;
        }
        record.used_chars += content_len;
        record.included.push(IncludedItem {
            asset_id: asset.asset_id.clone(),
            layer: asset.layer.clone(),
            chars: content_len,
            source: default_source(),
        });
        messages.push(ContextMessage {
            role: "system".to_string(),
            content: render_bootstrap_message(asset),
        });
    }
    (messages, record)
}

// ---------------------------------------------------------------------------
// Query-time retrieval (BM25 + recency, RRF fusion)
// ---------------------------------------------------------------------------

/// One retrieval candidate document. Callers pass the corpus **newest
/// first** — the index doubles as the recency rank.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RetrievalDoc {
    pub asset_id: String,
    pub title: String,
    pub content: String,
}

/// Lowercased unicode-word tokens (deterministic, language-naive; CJK text
/// splits on non-alphanumeric boundaries, which is coarse but predictable —
/// a vector ranker joins the fusion later for semantic recall).
fn tokenize(text: &str) -> Vec<String> {
    text.to_lowercase()
        .split(|c: char| !c.is_alphanumeric())
        .filter(|t| !t.is_empty())
        .map(ToString::to_string)
        .collect()
}

/// BM25 scores (k1=1.2, b=0.75) for `query` over `docs` (title + content).
/// Only docs with a positive score participate in retrieval.
fn bm25_scores(query: &str, docs: &[RetrievalDoc]) -> Vec<f64> {
    const K1: f64 = 1.2;
    const B: f64 = 0.75;
    let query_terms = tokenize(query);
    let doc_terms: Vec<Vec<String>> = docs
        .iter()
        .map(|d| tokenize(&format!("{} {}", d.title, d.content)))
        .collect();
    let n = docs.len() as f64;
    if n == 0.0 || query_terms.is_empty() {
        return vec![0.0; docs.len()];
    }
    let avgdl: f64 = doc_terms.iter().map(Vec::len).sum::<usize>() as f64 / n;
    let mut unique_terms: Vec<&String> = query_terms.iter().collect();
    unique_terms.sort();
    unique_terms.dedup();
    let mut scores = vec![0.0; docs.len()];
    for term in unique_terms {
        let df = doc_terms.iter().filter(|terms| terms.contains(term)).count() as f64;
        if df == 0.0 {
            continue;
        }
        let idf = (1.0 + (n - df + 0.5) / (df + 0.5)).ln();
        for (i, terms) in doc_terms.iter().enumerate() {
            let tf = terms.iter().filter(|t| *t == term).count() as f64;
            if tf == 0.0 {
                continue;
            }
            let dl = terms.len() as f64;
            let denom = tf + K1 * (1.0 - B + B * dl / avgdl.max(1.0));
            scores[i] += idf * (tf * (K1 + 1.0)) / denom;
        }
    }
    scores
}

/// Reciprocal-rank fusion (k=60) over two rankers — BM25 (positive scores
/// only) and recency (the caller's newest-first order). Returns candidate
/// doc indices, best first, capped at [`RETRIEVAL_MAX_CANDIDATES`]. Only
/// docs with a positive BM25 score are candidates (recency alone never
/// recalls unrelated memory). Deterministic: ties break on asset id.
#[must_use]
pub fn retrieve(query: &str, docs: &[RetrievalDoc]) -> Vec<usize> {
    const RRF_K: f64 = 60.0;
    let scores = bm25_scores(query, docs);
    let mut bm25_ranked: Vec<usize> = (0..docs.len()).filter(|&i| scores[i] > 0.0).collect();
    bm25_ranked.sort_by(|&a, &b| {
        scores[b]
            .partial_cmp(&scores[a])
            .unwrap_or(std::cmp::Ordering::Equal)
            .then_with(|| docs[a].asset_id.cmp(&docs[b].asset_id))
    });
    if bm25_ranked.is_empty() {
        return Vec::new();
    }
    let mut fused: Vec<(usize, f64)> = bm25_ranked
        .iter()
        .enumerate()
        .map(|(bm25_rank, &idx)| {
            // Recency rank = the caller's newest-first corpus index.
            let rrf = 1.0 / (RRF_K + bm25_rank as f64 + 1.0) + 1.0 / (RRF_K + idx as f64 + 1.0);
            (idx, rrf)
        })
        .collect();
    fused.sort_by(|a, b| {
        b.1.partial_cmp(&a.1)
            .unwrap_or(std::cmp::Ordering::Equal)
            .then_with(|| docs[a.0].asset_id.cmp(&docs[b.0].asset_id))
    });
    fused.truncate(RETRIEVAL_MAX_CANDIDATES);
    fused.into_iter().map(|(idx, _)| idx).collect()
}

/// Renders one recalled atom as its injected system message.
#[must_use]
pub fn render_recall_message(doc: &RetrievalDoc) -> String {
    let title = doc.title.trim();
    if title.is_empty() {
        format!("Memory[l1 {}] (recalled): {}", doc.asset_id, doc.content.trim())
    } else {
        format!(
            "Memory[l1 {}] {} (recalled): {}",
            doc.asset_id, title, doc.content.trim()
        )
    }
}

/// Runs retrieval for `query` over `docs` (newest first) and walks the
/// fused candidates under `budget_chars`, appending to `messages` and
/// `record` with `source: "retrieval"`. Deterministic.
pub fn retrieve_and_assemble(
    query: &str,
    docs: &[RetrievalDoc],
    budget_chars: usize,
    messages: &mut Vec<ContextMessage>,
    record: &mut AssemblyRecord,
) {
    record.retrieval_budget_chars = budget_chars;
    for idx in retrieve(query, docs) {
        let doc = &docs[idx];
        let content_len = doc.content.trim().len();
        if content_len == 0 {
            continue;
        }
        if record.retrieval_used_chars + content_len > budget_chars {
            record.excluded.push(ExcludedItem {
                asset_id: doc.asset_id.clone(),
                layer: "l1".to_string(),
                reason: "over_budget".to_string(),
                source: "retrieval".to_string(),
            });
            continue;
        }
        record.retrieval_used_chars += content_len;
        record.included.push(IncludedItem {
            asset_id: doc.asset_id.clone(),
            layer: "l1".to_string(),
            chars: content_len,
            source: "retrieval".to_string(),
        });
        messages.push(ContextMessage {
            role: "system".to_string(),
            content: render_recall_message(doc),
        });
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn asset(id: &str, layer: &str, content: &str) -> BootstrapAsset {
        BootstrapAsset {
            asset_id: id.to_string(),
            layer: layer.to_string(),
            title: String::new(),
            content: content.to_string(),
        }
    }

    #[test]
    fn includes_in_order_under_budget_with_citations() {
        let assets = vec![
            BootstrapAsset { title: "persona".to_string(), ..asset("mem_l3", "l3", "Chinese-speaking operator") },
            asset("mem_l2a", "l2", "prefers pnpm"),
        ];
        let (messages, record) = assemble(&assets, 1000);
        assert_eq!(messages.len(), 2);
        assert_eq!(
            messages[0].content,
            "Memory[l3 mem_l3] persona: Chinese-speaking operator"
        );
        assert_eq!(messages[1].content, "Memory[l2 mem_l2a]: prefers pnpm");
        assert!(messages.iter().all(|m| m.role == "system"));
        assert_eq!(record.included.len(), 2);
        assert!(record.excluded.is_empty());
        assert_eq!(record.used_chars, "Chinese-speaking operator".len() + "prefers pnpm".len());
    }

    #[test]
    fn over_budget_excludes_with_reason_but_keeps_walking() {
        let assets = vec![
            asset("big", "l3", &"x".repeat(100)),
            asset("small", "l2", "tiny"),
        ];
        let (messages, record) = assemble(&assets, 50);
        assert_eq!(messages.len(), 1, "the small asset still fits after the big one missed");
        assert!(messages[0].content.contains("small"));
        assert_eq!(record.excluded.len(), 1);
        assert_eq!(record.excluded[0].asset_id, "big");
        assert_eq!(record.excluded[0].reason, "over_budget");
    }

    #[test]
    fn empty_content_is_excluded_and_empty_pass_is_flagged() {
        let (messages, record) = assemble(&[asset("void", "l2", "   ")], 100);
        assert!(messages.is_empty());
        assert_eq!(record.excluded[0].reason, "empty_content");
        assert!(!record.is_empty());
        let (_, record) = assemble(&[], 100);
        assert!(record.is_empty());
    }

    fn doc(id: &str, content: &str) -> RetrievalDoc {
        RetrievalDoc { asset_id: id.to_string(), title: String::new(), content: content.to_string() }
    }

    #[test]
    fn bm25_ranks_term_overlap_and_zero_score_never_recalls() {
        let docs = vec![
            doc("mem_rust", "prefers rust for backend daemons"),
            doc("mem_lunch", "eats lunch at noon"),
            doc("mem_rustfmt", "rust formatting uses rustfmt defaults"),
        ];
        let picked = retrieve("which rust formatter do we use", &docs);
        assert!(!picked.is_empty());
        // Both rust docs are candidates; the lunch doc never appears.
        assert!(picked.iter().all(|&i| docs[i].asset_id != "mem_lunch"));
        assert!(picked.contains(&2), "doc with both `rust` and `formatting` terms recalled");

        // No lexical overlap at all: nothing is recalled (recency alone
        // never pulls unrelated memory in).
        assert!(retrieve("完全无关的查询", &docs).is_empty());
    }

    #[test]
    fn rrf_fusion_prefers_recent_on_equal_relevance_and_is_deterministic() {
        // Same content => same BM25 score; the newer doc (lower corpus
        // index) must win via the recency ranker.
        let docs = vec![doc("mem_new", "deploy checklist"), doc("mem_old", "deploy checklist")];
        let picked = retrieve("deploy checklist", &docs);
        assert_eq!(picked[0], 0, "newest-first corpus index wins on ties");
        assert_eq!(retrieve("deploy checklist", &docs), picked);
    }

    #[test]
    fn retrieve_and_assemble_records_source_and_budget() {
        let docs = vec![
            doc("mem_a", "pnpm is the package manager"),
            doc("mem_b", &format!("pnpm workspaces {}", "x".repeat(100))),
        ];
        let mut messages = Vec::new();
        let mut record = AssemblyRecord::default();
        retrieve_and_assemble("pnpm", &docs, 50, &mut messages, &mut record);
        assert_eq!(messages.len(), 1);
        assert!(messages[0].content.contains("Memory[l1 mem_a] (recalled):"));
        assert_eq!(record.included.len(), 1);
        assert_eq!(record.included[0].source, "retrieval");
        assert_eq!(record.excluded.len(), 1);
        assert_eq!(record.excluded[0].reason, "over_budget");
        assert_eq!(record.excluded[0].source, "retrieval");
        assert_eq!(record.retrieval_budget_chars, 50);
    }

    #[test]
    fn deterministic_and_config_defaults() {
        let assets = vec![asset("a", "l3", "one"), asset("b", "l2", "two")];
        assert_eq!(assemble(&assets, 100), assemble(&assets, 100));
        assert_eq!(ContextConfig::default().budget(), DEFAULT_MEMORY_BUDGET_CHARS);
        let parsed: ContextConfig =
            serde_json::from_value(serde_json::json!({ "memoryBudgetChars": 123 })).expect("parse");
        assert_eq!(parsed.budget(), 123);
    }
}
