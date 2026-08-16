use ratatui::{
    style::{Color, Modifier, Style},
    text::{Line, Span},
};

fn inline_spans(text: &str) -> Vec<Span<'static>> {
    let mut spans = Vec::new();
    let mut plain = String::new();
    let mut chars = text.char_indices().peekable();

    while let Some((_, c)) = chars.next() {
        if c == '*' && chars.peek().map(|(_, n)| *n) == Some('*') {
            if !plain.is_empty() {
                spans.push(Span::raw(std::mem::take(&mut plain)));
            }
            chars.next();
            let mut bold = String::new();
            while let Some((_, bc)) = chars.next() {
                if bc == '*' && chars.peek().map(|(_, n)| *n) == Some('*') {
                    chars.next();
                    break;
                }
                bold.push(bc);
            }
            spans.push(Span::styled(
                bold,
                Style::default().add_modifier(Modifier::BOLD),
            ));
        } else if c == '`' {
            if !plain.is_empty() {
                spans.push(Span::raw(std::mem::take(&mut plain)));
            }
            let mut code = String::new();
            for (_, cc) in chars.by_ref() {
                if cc == '`' {
                    break;
                }
                code.push(cc);
            }
            spans.push(Span::styled(code, Style::default().fg(Color::Cyan)));
        } else {
            plain.push(c);
        }
    }
    if !plain.is_empty() {
        spans.push(Span::raw(plain));
    }
    spans
}

pub fn markdown_lines(text: &str) -> Vec<Line<'static>> {
    let mut lines = Vec::new();
    let mut in_code = false;
    let mut code_buf = String::new();

    for raw in text.lines() {
        if raw.starts_with("```") {
            if in_code {
                for cl in code_buf.lines() {
                    lines.push(Line::from(Span::styled(
                        cl.to_string(),
                        Style::default().fg(Color::DarkGray),
                    )));
                }
                code_buf.clear();
                in_code = false;
            } else {
                in_code = true;
            }
            continue;
        }
        if in_code {
            code_buf.push_str(raw);
            code_buf.push('\n');
            continue;
        }

        let heading = if let Some(rest) = raw.strip_prefix("### ") {
            Some(rest)
        } else if let Some(rest) = raw.strip_prefix("## ") {
            Some(rest)
        } else {
            raw.strip_prefix("# ")
        };
        if let Some(rest) = heading {
            lines.push(Line::from(Span::styled(
                rest.to_string(),
                Style::default()
                    .fg(Color::White)
                    .add_modifier(Modifier::BOLD),
            )));
            continue;
        }

        if let Some(rest) = raw.strip_prefix("- ").or_else(|| raw.strip_prefix("* ")) {
            let mut spans = vec![Span::styled(
                "\u{2022} ",
                Style::default().fg(Color::DarkGray),
            )];
            spans.extend(inline_spans(rest));
            lines.push(Line::from(spans));
            continue;
        }

        if raw.is_empty() {
            lines.push(Line::from(""));
        } else {
            lines.push(Line::from(inline_spans(raw)));
        }
    }

    if in_code {
        for cl in code_buf.lines() {
            lines.push(Line::from(Span::styled(
                cl.to_string(),
                Style::default().fg(Color::DarkGray),
            )));
        }
    }

    lines
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn heading_is_bold() {
        let lines = markdown_lines("# Hello");
        assert_eq!(lines.len(), 1);
        assert!(
            lines[0].spans[0]
                .style
                .add_modifier
                .contains(Modifier::BOLD)
        );
        assert_eq!(lines[0].spans[0].content, "Hello");
    }

    #[test]
    fn code_block_renders_content() {
        let lines = markdown_lines("```\nlet x = 1;\n```");
        assert_eq!(lines.len(), 1);
        assert_eq!(lines[0].spans[0].content, "let x = 1;");
        assert_eq!(lines[0].spans[0].style.fg, Some(Color::DarkGray));
    }

    #[test]
    fn list_item_has_bullet() {
        let lines = markdown_lines("- item one");
        assert_eq!(lines.len(), 1);
        assert!(lines[0].spans[0].content.contains('\u{2022}'));
    }

    #[test]
    fn bold_inline_is_bold() {
        let lines = markdown_lines("hello **world**");
        assert_eq!(lines.len(), 1);
        assert!(
            lines[0]
                .spans
                .iter()
                .any(|s| s.style.add_modifier.contains(Modifier::BOLD)
                    && s.content.contains("world"))
        );
    }

    #[test]
    fn inline_code_is_cyan() {
        let lines = markdown_lines("run `cargo test` now");
        assert_eq!(lines.len(), 1);
        assert!(
            lines[0]
                .spans
                .iter()
                .any(|s| s.content.contains("cargo test") && s.style.fg == Some(Color::Cyan))
        );
    }

    #[test]
    fn plain_paragraph_is_plain() {
        let lines = markdown_lines("just some text");
        assert_eq!(lines.len(), 1);
        assert_eq!(lines[0].spans[0].content, "just some text");
    }
}
