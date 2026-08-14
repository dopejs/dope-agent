use std::io::Write;
use std::io::stdout;
use std::process::ExitCode;
use std::sync::Arc;

use clap::Parser;
use clap::Subcommand;
use dope_core::Session;
use dope_core::ToolRegistry;
use dope_model_provider::OpenAiCompatibleClient;
use dope_protocol::EventMsg;

/// DopeAgent Rust client — drives the agent core against an
/// OpenAI-compatible provider endpoint.
#[derive(Parser)]
#[command(name = "dope-cli", version, about)]
struct Cli {
    #[command(subcommand)]
    command: Command,
}

#[derive(Subcommand)]
enum Command {
    /// Run a single turn with the given prompt and print the reply.
    Exec {
        /// User prompt for the turn.
        prompt: String,
        /// Base URL of an OpenAI-compatible endpoint.
        #[arg(
            long,
            env = "DOPE_BASE_URL",
            default_value = "https://api.openai.com/v1"
        )]
        base_url: String,
        /// Model identifier.
        #[arg(long, env = "DOPE_MODEL", default_value = "gpt-4o-mini")]
        model: String,
        /// API key; falls back to OPENAI_API_KEY.
        #[arg(long, env = "DOPE_API_KEY")]
        api_key: Option<String>,
        /// System instructions for the session.
        #[arg(long)]
        instructions: Option<String>,
    },
}

#[tokio::main]
async fn main() -> ExitCode {
    let cli = Cli::parse();
    match cli.command {
        Command::Exec {
            prompt,
            base_url,
            model,
            api_key,
            instructions,
        } => {
            let api_key = api_key.or_else(|| std::env::var("OPENAI_API_KEY").ok());
            let provider = Arc::new(OpenAiCompatibleClient::new(base_url, model, api_key));
            let mut session = Session::new(provider, ToolRegistry::new());
            if let Some(instructions) = instructions {
                session = session.with_instructions(instructions);
            }
            let result = session
                .run_turn(&prompt, &mut |event| match event.msg {
                    EventMsg::AgentMessageDelta { delta } => {
                        print!("{delta}");
                        let _ = stdout().flush();
                    }
                    EventMsg::ToolCallBegin { name, .. } => {
                        eprintln!("[tool] {name}");
                    }
                    EventMsg::Error { message } => {
                        eprintln!("[error] {message}");
                    }
                    _ => {}
                })
                .await;
            match result {
                Ok(_) => {
                    println!();
                    ExitCode::SUCCESS
                }
                Err(err) => {
                    eprintln!("turn failed: {err}");
                    ExitCode::FAILURE
                }
            }
        }
    }
}
