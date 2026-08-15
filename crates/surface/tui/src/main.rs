mod client;
mod render;

use std::io;
use std::sync::Arc;

use clap::Parser;
use ratatui::{
    backend::CrosstermBackend,
    crossterm::{
        event::{self, Event, KeyCode, KeyModifiers},
        execute,
        terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
    },
    layout::{Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph},
    Terminal,
};
use tokio::sync::mpsc;

use client::{ChatQueryInput, ChatQueryResponse, Client};

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

#[derive(Clone, Copy, PartialEq)]
enum Role { User, Assistant, System, Error }

struct Message {
    role: Role,
    content: String,
    done: bool,
}

struct App {
    daemon_url: String,
    provider: Option<String>,
    model: Option<String>,
    thread_id: Option<String>,
    input: String,
    messages: Vec<Message>,
    history: Vec<String>,
    history_index: i32,
    scroll_offset: usize,
    busy: bool,
}

enum StreamMsg {
    Delta(String),
    Done(Result<ChatQueryResponse, String>),
}

impl App {
    fn push(&mut self, role: Role, content: String, done: bool) {
        self.messages.push(Message { role, content, done });
        self.scroll_offset = 0;
    }

    fn status(&self) -> String {
        format!(
            "{}  provider={} model={} thread={}",
            self.daemon_url,
            self.provider.as_deref().unwrap_or("default"),
            self.model.as_deref().unwrap_or("default"),
            self.thread_id.as_deref().unwrap_or("new"),
        )
    }
}

fn draw(frame: &mut ratatui::Frame, app: &App) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([Constraint::Length(1), Constraint::Min(3), Constraint::Length(3)])
        .split(frame.area());

    frame.render_widget(Paragraph::new(app.status()).style(Style::default().fg(Color::DarkGray)), chunks[0]);

    let visible_count = chunks[1].height.saturating_sub(1) as usize;
    let total = app.messages.len();
    let start = if app.scroll_offset >= total { 0 } else { total - app.scroll_offset - visible_count.min(total - app.scroll_offset) };
    let visible = &app.messages[start..];

    let mut body: Vec<Line<'static>> = Vec::new();
    for m in visible {
        match m.role {
            Role::User => {
                let mut spans = vec![Span::styled("> ", Style::default().fg(Color::Green).add_modifier(Modifier::BOLD))];
                spans.push(Span::styled(m.content.clone(), Style::default().fg(Color::Green)));
                body.push(Line::from(spans));
            }
            Role::Assistant => body.extend(render::markdown_lines(&m.content)),
            Role::Error => {
                body.push(Line::from(Span::styled(format!("[error] {}", m.content), Style::default().fg(Color::Red))));
            }
            Role::System => {
                body.push(Line::from(Span::styled(m.content.clone(), Style::default().fg(Color::DarkGray))));
            }
        }
    }
    if app.busy {
        body.push(Line::from(Span::styled("\u{258c}", Style::default().fg(Color::DarkGray))));
    }
    frame.render_widget(Paragraph::new(body), chunks[1]);

    let prompt = format!("\u{276f} {}", app.input);
    frame.render_widget(
        Paragraph::new(prompt).block(Block::default().borders(Borders::ALL).border_style(Style::default().fg(Color::Blue))),
        chunks[2],
    );
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let cli = Cli::parse();

    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    let client = Arc::new(Client::new(cli.daemon_url.clone(), cli.token.clone()));
    let mut app = App {
        daemon_url: cli.daemon_url,
        provider: cli.provider,
        model: cli.model,
        thread_id: None,
        input: String::new(),
        messages: vec![Message { role: Role::System, content: "DopeAgent (type /help)".to_string(), done: true }],
        history: Vec::new(),
        history_index: -1,
        scroll_offset: 0,
        busy: false,
    };

    // crossterm events -> channel (blocking read on a worker thread)
    let (event_tx, mut event_rx) = mpsc::channel::<Event>(32);
    tokio::task::spawn_blocking(move || loop {
        match event::read() {
            Ok(ev) => { if event_tx.blocking_send(ev).is_err() { break; } }
            Err(_) => break,
        }
    });

    // streaming deltas -> channel
    let (stream_tx, mut stream_rx) = mpsc::channel::<StreamMsg>(64);

    loop {
        terminal.draw(|f| draw(f, &app))?;

        tokio::select! {
            Some(ev) = event_rx.recv() => {
                let quit = handle_key(&mut app, ev, &client, &stream_tx);
                if quit { break; }
            }
            Some(msg) = stream_rx.recv() => {
                match msg {
                    StreamMsg::Delta(d) => {
                        if let Some(last) = app.messages.last_mut() {
                            last.content.push_str(&d);
                        }
                    }
                    StreamMsg::Done(result) => {
                        app.busy = false;
                        if let Some(last) = app.messages.last_mut() {
                            last.done = true;
                        }
                        match result {
                            Ok(resp) => {
                                if resp.thread_id.is_some() {
                                    app.thread_id = resp.thread_id;
                                }
                                if let Some(last) = app.messages.last_mut() {
                                    if last.content.is_empty() { last.content = resp.reply; }
                                }
                            }
                            Err(e) => {
                                app.push(Role::Error, e, true);
                            }
                        }
                    }
                }
            }
        }
    }

    disable_raw_mode()?;
    execute!(terminal.backend_mut(), LeaveAlternateScreen)?;
    Ok(())
}

fn handle_key(app: &mut App, ev: Event, client: &Arc<Client>, stream_tx: &mpsc::Sender<StreamMsg>) -> bool {
    let Event::Key(key) = ev else { return false };
    if key.modifiers.contains(KeyModifiers::CONTROL) && key.code == KeyCode::Char('c') {
        return true;
    }
    match key.code {
        KeyCode::Esc => {
            if app.busy { app.busy = false; } else { return true; }
        }
        KeyCode::Enter => {
            submit(app, client, stream_tx);
        }
        KeyCode::Backspace => { app.input.pop(); }
        KeyCode::Up => {
            if app.history.is_empty() { return false; }
            app.history_index = if app.history_index < 0 { app.history.len() as i32 - 1 } else { (app.history_index - 1).max(0) };
            app.input = app.history[app.history_index as usize].clone();
        }
        KeyCode::Down => {
            if app.history_index < 0 { return false; }
            app.history_index += 1;
            if app.history_index as usize >= app.history.len() { app.history_index = -1; app.input.clear(); }
            else { app.input = app.history[app.history_index as usize].clone(); }
        }
        KeyCode::PageUp => { app.scroll_offset = app.scroll_offset.saturating_add(10); }
        KeyCode::PageDown => { app.scroll_offset = app.scroll_offset.saturating_sub(10); }
        KeyCode::Char(c) => app.input.push(c),
        _ => {}
    }
    false
}

fn submit(app: &mut App, client: &Arc<Client>, stream_tx: &mpsc::Sender<StreamMsg>) {
    let text = app.input.trim().to_string();
    if text.is_empty() || app.busy { return; }
    app.input.clear();
    app.history.push(text.clone());
    app.history_index = -1;

    if text.starts_with('/') {
        app.push(Role::System, format!("(no command support yet) {text}"), true);
        return;
    }

    app.push(Role::User, text.clone(), true);
    app.push(Role::Assistant, String::new(), false);
    app.busy = true;

    let input = ChatQueryInput {
        query: text,
        provider: app.provider.clone(),
        model: app.model.clone(),
        thread_id: app.thread_id.clone(),
    };
    let client = client.clone();
    let tx = stream_tx.clone();
    tokio::spawn(async move {
        let result = client.stream_chat(&input, |delta| {
            let _ = tx.try_send(StreamMsg::Delta(delta));
        }).await;
        let _ = tx.send(StreamMsg::Done(result)).await;
    });
}