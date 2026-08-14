//! dope-cli — daemon entry point (port of daemon/cmd/dope/main.go).
//!
//! Loads the effective config (DOPE_ENV / ~/.dope-test|~/.dope + config.json),
//! builds the daemon application (dope-app), serves the HTTP API until
//! SIGINT/SIGTERM (handled inside App::serve), and exits non-zero on any
//! fatal error.

use std::process::ExitCode;
use std::sync::Arc;

#[tokio::main]
async fn main() -> ExitCode {
    match run().await {
        Ok(()) => ExitCode::SUCCESS,
        Err(err) => {
            eprintln!("dope: fatal: {err}");
            ExitCode::FAILURE
        }
    }
}

async fn run() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let config = dope_config::load()?;
    let app = Arc::new(dope_app::App::new(config)?);
    app.serve().await?;
    Ok(())
}
