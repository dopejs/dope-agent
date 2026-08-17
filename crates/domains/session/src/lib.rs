//! dope-session — session-strategy policies for the plugin plane.
//!
//! The user-facing thesis: for a personal agent, context management is
//! session management, and it belongs to plugins. This crate is the policy
//! engine behind the `session-strategy` builtin plugin, which attaches at
//! the `chat/pre-dispatch` hook and shapes the assembled message window
//! **deterministically** before the dispatch is prepared (and therefore
//! before it is persisted — the shaped window is exactly what is logged and
//! what the model sees).
//!
//! Two strategies share one mechanism and differ by budget:
//! - **personal** (default): long-session window for direct chat/shell work.
//! - **thread**: tighter window for channel-origin turns (one context per
//!   thread; thread scoping itself comes from the continuity plane).
//!
//! The mechanism is frame-preserving elision:
//! - system messages are the **frame** (persona, skills, safety posture) —
//!   never elided;
//! - the most recent `keep_recent` non-system messages are always kept
//!   (the current query is the last of them);
//! - when the total content length exceeds the strategy's budget, oldest
//!   non-system messages are elided and replaced with a single marker line.
//!
//! Eviction is safe by construction: every chat turn is captured to the
//! memory plane (L0) at turn settle independently of the window, so elided
//! turns remain reachable through thread continuity and memory — the marker
//! says so to the model.

use serde::{Deserialize, Serialize};

/// The role/content shape of one message in the hook payload (a structural
/// subset of `dope_llm::Message`; this crate stays decoupled from the LLM
/// crate on purpose).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WindowMessage {
    pub role: String,
    pub content: String,
}

/// Plugin configuration (the `config` object of the `session-strategy`
/// entry in `plugins.json`). Zero values fall back to defaults.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct SessionStrategyConfig {
    /// Budget for personal (non-channel) turns, in content chars.
    pub personal_budget_chars: usize,
    /// Budget for channel-origin turns, in content chars.
    pub thread_budget_chars: usize,
    /// Minimum number of most-recent non-system messages always kept.
    pub keep_recent: usize,
}

/// Personal-session default budget (~a long working window).
pub const DEFAULT_PERSONAL_BUDGET_CHARS: usize = 48_000;
/// Thread default budget (IM threads stay tight and pure).
pub const DEFAULT_THREAD_BUDGET_CHARS: usize = 16_000;
/// Default floor of recent non-system messages.
pub const DEFAULT_KEEP_RECENT: usize = 4;

impl SessionStrategyConfig {
    /// Effective budget for a turn given its source kind (`channel` uses the
    /// thread budget; everything else is personal).
    #[must_use]
    pub fn budget_for(&self, source_kind: Option<&str>) -> usize {
        let is_channel = source_kind == Some("channel");
        if is_channel {
            if self.thread_budget_chars > 0 {
                self.thread_budget_chars
            } else {
                DEFAULT_THREAD_BUDGET_CHARS
            }
        } else if self.personal_budget_chars > 0 {
            self.personal_budget_chars
        } else {
            DEFAULT_PERSONAL_BUDGET_CHARS
        }
    }

    #[must_use]
    pub fn keep_recent_floor(&self) -> usize {
        if self.keep_recent > 0 {
            self.keep_recent
        } else {
            DEFAULT_KEEP_RECENT
        }
    }
}

/// Result of one window-shaping pass.
#[derive(Debug, Clone, PartialEq)]
pub struct ShapedWindow {
    pub messages: Vec<WindowMessage>,
    /// Number of non-system messages elided (0 = untouched).
    pub elided: usize,
}

/// The marker inserted where messages were elided.
#[must_use]
pub fn elision_marker(elided: usize) -> String {
    format!(
        "[session window: {elided} earlier message(s) elided by the session-strategy plugin; \
         the full history remains in thread continuity and the memory plane]"
    )
}

/// Shapes `messages` to fit `budget_chars` of total content, preserving the
/// frame (system messages) and at least `keep_recent` most-recent
/// non-system messages. Deterministic: same input → same output. When the
/// input fits the budget the messages come back untouched.
#[must_use]
pub fn shape_window(
    messages: &[WindowMessage],
    budget_chars: usize,
    keep_recent: usize,
) -> ShapedWindow {
    let total: usize = messages.iter().map(|m| m.content.len()).sum();
    if total <= budget_chars {
        return ShapedWindow { messages: messages.to_vec(), elided: 0 };
    }

    // Frame chars are mandatory; history fits in what remains.
    let frame_chars: usize = messages
        .iter()
        .filter(|m| m.role == "system")
        .map(|m| m.content.len())
        .sum();
    let history_budget = budget_chars.saturating_sub(frame_chars);

    // Walk non-system messages from newest to oldest, keeping while within
    // the history budget; `keep_recent` newest are kept unconditionally.
    let history_indices: Vec<usize> = messages
        .iter()
        .enumerate()
        .filter(|(_, m)| m.role != "system")
        .map(|(i, _)| i)
        .collect();
    let mut kept: std::collections::HashSet<usize> = std::collections::HashSet::new();
    let mut used = 0usize;
    for (rank, &idx) in history_indices.iter().rev().enumerate() {
        let len = messages[idx].content.len();
        if rank < keep_recent || used + len <= history_budget {
            kept.insert(idx);
            used += len;
        }
    }

    let elided = history_indices.len() - kept.len();
    if elided == 0 {
        return ShapedWindow { messages: messages.to_vec(), elided: 0 };
    }

    // Rebuild: frame stays in place; the marker replaces the first elided
    // position so the model sees where history was cut.
    let mut out: Vec<WindowMessage> = Vec::with_capacity(messages.len() - elided + 1);
    let mut marker_placed = false;
    for (idx, message) in messages.iter().enumerate() {
        if message.role == "system" || kept.contains(&idx) {
            out.push(message.clone());
        } else if !marker_placed {
            out.push(WindowMessage {
                role: "system".to_string(),
                content: elision_marker(elided),
            });
            marker_placed = true;
        }
    }
    ShapedWindow { messages: out, elided }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn msg(role: &str, content: &str) -> WindowMessage {
        WindowMessage { role: role.to_string(), content: content.to_string() }
    }

    #[test]
    fn under_budget_is_untouched() {
        let messages = vec![msg("system", "frame"), msg("user", "hello")];
        let shaped = shape_window(&messages, 1000, 4);
        assert_eq!(shaped.elided, 0);
        assert_eq!(shaped.messages, messages);
    }

    #[test]
    fn over_budget_elides_oldest_history_and_keeps_frame() {
        let mut messages = vec![msg("system", "persona frame")];
        for i in 0..10 {
            messages.push(msg("user", &format!("question {i} {}", "x".repeat(100))));
            messages.push(msg("assistant", &format!("answer {i} {}", "y".repeat(100))));
        }
        messages.push(msg("user", "current query"));
        let shaped = shape_window(&messages, 500, 2);
        assert!(shaped.elided > 0);
        // Frame preserved.
        assert!(shaped.messages.iter().any(|m| m.content == "persona frame"));
        // Current query preserved (keep_recent floor).
        assert_eq!(shaped.messages.last().unwrap().content, "current query");
        // Marker present exactly once, positioned before the kept history.
        let markers: Vec<usize> = shaped
            .messages
            .iter()
            .enumerate()
            .filter(|(_, m)| m.content.contains("elided by the session-strategy"))
            .map(|(i, _)| i)
            .collect();
        assert_eq!(markers.len(), 1);
        // Oldest history gone, newest kept.
        assert!(!shaped.messages.iter().any(|m| m.content.starts_with("question 0")));
        // Deterministic.
        let again = shape_window(&messages, 500, 2);
        assert_eq!(again, shaped);
    }

    #[test]
    fn keep_recent_floor_wins_over_budget() {
        let messages = vec![
            msg("user", &"a".repeat(300)),
            msg("assistant", &"b".repeat(300)),
            msg("user", &"c".repeat(300)),
        ];
        // Budget of zero still keeps the floor.
        let shaped = shape_window(&messages, 0, 2);
        assert_eq!(shaped.elided, 1);
        let non_system: Vec<&WindowMessage> =
            shaped.messages.iter().filter(|m| m.role != "system").collect();
        assert_eq!(non_system.len(), 2);
        assert!(non_system[1].content.starts_with('c'));
    }

    #[test]
    fn config_budgets_key_off_source_kind() {
        let config = SessionStrategyConfig::default();
        assert_eq!(config.budget_for(None), DEFAULT_PERSONAL_BUDGET_CHARS);
        assert_eq!(config.budget_for(Some("chat")), DEFAULT_PERSONAL_BUDGET_CHARS);
        assert_eq!(config.budget_for(Some("channel")), DEFAULT_THREAD_BUDGET_CHARS);
        let custom = SessionStrategyConfig {
            personal_budget_chars: 100,
            thread_budget_chars: 50,
            keep_recent: 1,
        };
        assert_eq!(custom.budget_for(None), 100);
        assert_eq!(custom.budget_for(Some("channel")), 50);
        assert_eq!(custom.keep_recent_floor(), 1);

        // Parses from a plugins.json config object.
        let parsed: SessionStrategyConfig = serde_json::from_value(serde_json::json!({
            "personalBudgetChars": 200,
            "threadBudgetChars": 80,
            "keepRecent": 3
        }))
        .expect("parse");
        assert_eq!(parsed.budget_for(None), 200);
    }
}
