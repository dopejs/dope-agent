//! Model provider abstraction and concrete provider clients.
//!
//! Mirrors `codex-rs/model-provider`: the core talks to `dyn ModelProvider`
//! and never to HTTP directly, so provider quirks stay in this crate.

mod openai;
mod provider;

pub use openai::OpenAiCompatibleClient;
pub use provider::{ModelProvider, Prompt, ProviderError, ResponseEvent, ToolSpec};
