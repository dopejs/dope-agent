use std::sync::Arc;

use crate::client::Client;
use crate::Role;

pub enum CommandResult {
    Push(Role, String),
    SetModel(Option<String>),
    SetProvider(Option<String>),
    SetThread(Option<String>),
    Quit,
}

const HELP: &str = "Commands:\n\
  /help                  Show this help\n\
  /exit, /quit           Quit\n\
  /model [name]          Set the model (empty to clear)\n\
  /provider [name]       Set the provider (empty to clear)\n\
  /thread [id]           Set the active thread (empty to clear)\n\
  /threads               List threads\n\
  /reset <thread-id>     Reset a thread\n\
  /workspaces            List workspaces\n\
  /bindings              List workspace bindings\n\
  /profiles              List agent profiles\n\
  /connectors            List channel connectors\n\
  /tenants               List tenants\n\
  /me                    Show the authenticated principal\n\
  /config                Show daemon config\n";

fn opt(s: &str) -> Option<String> {
    let t = s.trim();
    if t.is_empty() { None } else { Some(t.to_string()) }
}

fn pretty(v: &serde_json::Value) -> String {
    serde_json::to_string_pretty(v).unwrap_or_else(|_| String::new())
}

pub async fn run_command(cmd: &str, args: &str, client: &Arc<Client>) -> CommandResult {
    match cmd {
        "/help" => CommandResult::Push(Role::System, HELP.to_string()),
        "/exit" | "/quit" => CommandResult::Quit,
        "/model" => CommandResult::SetModel(opt(args)),
        "/provider" => CommandResult::SetProvider(opt(args)),
        "/thread" => CommandResult::SetThread(opt(args)),
        _ => {
            let path = match cmd {
                "/threads" => "/v1/threads",
                "/workspaces" => "/v1/workspaces",
                "/bindings" => "/v1/bindings",
                "/profiles" => "/v1/profiles",
                "/connectors" => "/v1/channel-management/connectors",
                "/tenants" => "/v1/tenants",
                "/me" => "/v1/auth/me",
                "/config" => "/v1/system/info",
                _ => {
                    return CommandResult::Push(Role::Error, format!("unknown command {cmd}. Type /help."));
                }
            };
            match client.get_json(path).await {
                Ok(v) => CommandResult::Push(Role::System, pretty(&v)),
                Err(e) => CommandResult::Push(Role::Error, format!("[error] {e}")),
            }
        }
    }
}
