use std::io;

use clap::Parser;
use ratatui::{
    backend::CrosstermBackend,
    crossterm::{
        event::{self, Event, KeyCode, KeyModifiers},
        execute,
        terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
    },
    layout::{Constraint, Direction, Layout},
    widgets::{Block, Borders, Paragraph},
    Terminal,
};

/// DopeAgent full-screen terminal client.
#[derive(Parser, Debug)]
#[command(name = "dope-tui", version)]
struct Cli {
    /// Daemon base URL.
    #[arg(long, env = "DOPE_DAEMON_URL", default_value = "http://127.0.0.1:19192")]
    daemon_url: String,
    /// Access token for daemon auth.
    #[arg(long, env = "DOPE_ACCESS_TOKEN")]
    token: Option<String>,
    /// Optional provider override.
    #[arg(long)]
    provider: Option<String>,
    /// Optional model override.
    #[arg(long)]
    model: Option<String>,
}

struct App {
    daemon_url: String,
    provider: Option<String>,
    model: Option<String>,
    thread_id: Option<String>,
    input: String,
    messages: Vec<String>,
}

fn draw(frame: &mut ratatui::Frame, app: &App) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([Constraint::Length(1), Constraint::Min(3), Constraint::Length(3)])
        .split(frame.area());

    let status = format!(
        "{}  provider={} model={} thread={}",
        app.daemon_url,
        app.provider.as_deref().unwrap_or("default"),
        app.model.as_deref().unwrap_or("default"),
        app.thread_id.as_deref().unwrap_or("new"),
    );
    frame.render_widget(Paragraph::new(status), chunks[0]);

    let body = app.messages.join("\n");
    frame.render_widget(Paragraph::new(body).block(Block::default()), chunks[1]);

    let prompt = format!("\u{276f} {}", app.input);
    frame.render_widget(
        Paragraph::new(prompt).block(Block::default().borders(Borders::ALL)),
        chunks[2],
    );
}

fn main() -> io::Result<()> {
    let cli = Cli::parse();

    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    let mut app = App {
        daemon_url: cli.daemon_url,
        provider: cli.provider,
        model: cli.model,
        thread_id: None,
        input: String::new(),
        messages: vec!["DopeAgent TUI (type /help)".to_string()],
    };

    loop {
        terminal.draw(|f| draw(f, &app))?;
        if let Event::Key(key) = event::read()? {
            if key.modifiers.contains(KeyModifiers::CONTROL) && key.code == KeyCode::Char('c') {
                break;
            }
            match key.code {
                KeyCode::Esc => break,
                KeyCode::Enter => {
                    let text = app.input.trim().to_string();
                    if !text.is_empty() {
                        app.messages.push(format!("> {text}"));
                    }
                    app.input.clear();
                }
                KeyCode::Backspace => {
                    app.input.pop();
                }
                KeyCode::Char(c) => app.input.push(c),
                _ => {}
            }
        }
    }

    disable_raw_mode()?;
    execute!(terminal.backend_mut(), LeaveAlternateScreen)?;
    Ok(())
}