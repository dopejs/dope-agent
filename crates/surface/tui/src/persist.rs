use std::fs;
use std::path::PathBuf;

use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct PersistedMessage {
    pub role: String,
    pub content: String,
}

#[derive(Serialize, Deserialize, Clone, Debug, Default)]
pub struct PersistedState {
    pub messages: Vec<PersistedMessage>,
    pub provider: Option<String>,
    pub model: Option<String>,
    pub thread_id: Option<String>,
}

pub fn state_path() -> PathBuf {
    let dir = std::env::var("KURA_TUI_STATE_DIR").unwrap_or_else(|_| {
        let home = std::env::var("HOME").unwrap_or_else(|_| ".".to_string());
        format!("{home}/.kura-tui")
    });
    PathBuf::from(dir).join("state.json")
}

pub fn load() -> Option<PersistedState> {
    let raw = fs::read_to_string(state_path()).ok()?;
    serde_json::from_str(&raw).ok()
}

pub fn save(state: &PersistedState) -> Result<(), String> {
    let path = state_path();
    if let Some(dir) = path.parent() {
        fs::create_dir_all(dir).map_err(|e| e.to_string())?;
    }
    let json = serde_json::to_string_pretty(state).map_err(|e| e.to_string())?;
    fs::write(&path, json).map_err(|e| e.to_string())
}
