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
    /// Total character budget for injected memory content.
    pub memory_budget_chars: usize,
}

/// Default memory bootstrap budget (chars of asset content).
pub const DEFAULT_MEMORY_BUDGET_CHARS: usize = 4000;

impl ContextConfig {
    #[must_use]
    pub fn budget(&self) -> usize {
        if self.memory_budget_chars > 0 {
            self.memory_budget_chars
        } else {
            DEFAULT_MEMORY_BUDGET_CHARS
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

/// One included asset in the assembly record.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct IncludedItem {
    pub asset_id: String,
    pub layer: String,
    pub chars: usize,
}

/// One excluded asset and why.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ExcludedItem {
    pub asset_id: String,
    pub layer: String,
    /// `over_budget` | `empty_content` | `visibility`.
    pub reason: String,
}

/// The full decision record of one assembly pass.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AssemblyRecord {
    pub included: Vec<IncludedItem>,
    pub excluded: Vec<ExcludedItem>,
    pub budget_chars: usize,
    pub used_chars: usize,
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
            });
            continue;
        }
        if record.used_chars + content_len > budget_chars {
            record.excluded.push(ExcludedItem {
                asset_id: asset.asset_id.clone(),
                layer: asset.layer.clone(),
                reason: "over_budget".to_string(),
            });
            continue;
        }
        record.used_chars += content_len;
        record.included.push(IncludedItem {
            asset_id: asset.asset_id.clone(),
            layer: asset.layer.clone(),
            chars: content_len,
        });
        messages.push(ContextMessage {
            role: "system".to_string(),
            content: render_bootstrap_message(asset),
        });
    }
    (messages, record)
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
