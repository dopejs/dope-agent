use std::sync::Arc;

use crate::Role;
use crate::client::Client;

#[derive(Debug)]
pub enum CommandResult {
    Push(Role, String),
    SetModel(Option<String>),
    SetProvider(Option<String>),
    SetThread(Option<String>),
    /// Open an interactive single-column picker; the selected value becomes the active thread.
    OpenThreadPicker {
        title: String,
        items: Vec<(String, String)>,
    },
    /// Toggle the live daemon event stream (steps / tool calls / dispatch).
    ToggleEvents,
    Quit,
}

const HELP: &str = "Commands:\n\
  /help                  Show this help\n\
  /exit, /quit           Quit\n\
  /model [name]          Set the model (empty to clear)\n\
  /provider [name]       Set the provider (empty to clear)\n\
  /thread [id]           Set the active thread (empty to clear)\n\
  /threads               List threads (interactive picker)\n\
  /reset <thread-id>     Reset a thread\n\
  /workspaces            List workspaces\n\
  /bindings              List workspace bindings\n\
  /profiles              List agent profiles\n\
  /connectors            List channel connectors\n\
  /tenants               List tenants\n\
  /me                    Show the authenticated principal\n\
  /config                Show daemon config\n\
  /events               Stream daemon events (steps / tool calls)\n\n\
Keys:\n\
  Enter           Send; trailing \\ continues to the next line\n\
  Ctrl+X          Edit prompt in $EDITOR\n\
  Ctrl+L          Clear transcript\n\
  Up/Down         History; PageUp/PageDown scroll\n\
  Tab             Complete slash command\n\
  Esc             Cancel / quit";

fn opt(s: &str) -> Option<String> {
    let t = s.trim();
    if t.is_empty() {
        None
    } else {
        Some(t.to_string())
    }
}

fn pretty(v: &serde_json::Value) -> String {
    serde_json::to_string_pretty(v).unwrap_or_else(|_| String::new())
}

pub async fn run_command(cmd: &str, args: &str, client: &Arc<Client>) -> CommandResult {
    match cmd {
        "/help" => CommandResult::Push(Role::System, HELP.to_string()),
        "/events" => CommandResult::ToggleEvents,
        "/exit" | "/quit" => CommandResult::Quit,
        "/model" => CommandResult::SetModel(opt(args)),
        "/provider" => CommandResult::SetProvider(opt(args)),
        "/thread" => CommandResult::SetThread(opt(args)),
        "/threads" => match client.get_json("/v1/threads").await {
            Ok(v) => {
                let items = v
                    .get("items")
                    .and_then(|a| a.as_array())
                    .map(|a| {
                        a.iter()
                            .filter_map(|t| {
                                let id = t.get("threadId").and_then(|s| s.as_str()).unwrap_or("");
                                if id.is_empty() {
                                    None
                                } else {
                                    let label = format!(
                                        "{}  {}  {}",
                                        id,
                                        t.get("lifecycleState")
                                            .and_then(|s| s.as_str())
                                            .unwrap_or(""),
                                        t.get("sourceKind").and_then(|s| s.as_str()).unwrap_or(""),
                                    );
                                    Some((id.to_string(), label))
                                }
                            })
                            .collect::<Vec<_>>()
                    })
                    .unwrap_or_default();
                if items.is_empty() {
                    CommandResult::Push(Role::System, "No threads.".to_string())
                } else {
                    CommandResult::OpenThreadPicker {
                        title: "Select a thread".to_string(),
                        items,
                    }
                }
            }
            Err(e) => CommandResult::Push(Role::Error, format!("[error] {e}")),
        },
        _ => {
            let path = match cmd {
                "/workspaces" => "/v1/workspaces",
                "/bindings" => "/v1/bindings",
                "/profiles" => "/v1/profiles",
                "/connectors" => "/v1/channel-management/connectors",
                "/tenants" => "/v1/tenants",
                "/me" => "/v1/auth/me",
                "/config" => "/v1/system/info",
                _ => {
                    return CommandResult::Push(
                        Role::Error,
                        format!("unknown command {cmd}. Type /help."),
                    );
                }
            };
            match client.get_json(path).await {
                Ok(v) => CommandResult::Push(Role::System, pretty(&v)),
                Err(e) => CommandResult::Push(Role::Error, format!("[error] {e}")),
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::Role;

    fn client() -> Arc<Client> {
        Arc::new(Client::new("http://127.0.0.1:1".to_string(), None))
    }

    #[tokio::test]
    async fn help_returns_help() {
        match run_command("/help", "", &client()).await {
            CommandResult::Push(Role::System, content) => assert!(content.contains("/threads")),
            other => panic!("expected Push(System), got {other:?}"),
        }
    }

    #[tokio::test]
    async fn exit_quits() {
        assert!(matches!(
            run_command("/exit", "", &client()).await,
            CommandResult::Quit
        ));
    }

    #[tokio::test]
    async fn model_sets_model() {
        match run_command("/model", "gpt-4o", &client()).await {
            CommandResult::SetModel(Some(m)) => assert_eq!(m, "gpt-4o"),
            other => panic!("expected SetModel, got {other:?}"),
        }
    }

    #[tokio::test]
    async fn model_clears_when_empty() {
        assert!(matches!(
            run_command("/model", "", &client()).await,
            CommandResult::SetModel(None)
        ));
    }

    #[tokio::test]
    async fn unknown_command_errors() {
        match run_command("/bogus", "", &client()).await {
            CommandResult::Push(Role::Error, content) => {
                assert!(content.contains("unknown command"))
            }
            other => panic!("expected Push(Error), got {other:?}"),
        }
    }
}
