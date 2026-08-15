import { Text, Box } from "ink";
import { marked } from "marked";
import type { ReactNode } from "react";

// marked token types are a large discriminated union with fiddly optional fields;
// this leaf renderer treats them loosely so the switch below stays simple.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Token = any;

export function Inline({ tokens }: { tokens: Token[] }) {
  const children: ReactNode[] = [];
  for (let i = 0; i < tokens.length; i += 1) {
    const t = tokens[i];
    switch (t.type) {
      case "strong":
        children.push(<Text key={i} bold><Inline tokens={t.tokens ?? []} /></Text>);
        break;
      case "em":
        children.push(<Text key={i} italic><Inline tokens={t.tokens ?? []} /></Text>);
        break;
      case "codespan":
        children.push(<Text key={i} color="cyan">{t.text}</Text>);
        break;
      case "link":
        children.push(<Text key={i} color="blue" underline>{t.text}</Text>);
        break;
      case "br":
        children.push("\n");
        break;
      case "del":
        children.push(<Text key={i} strikethrough><Inline tokens={t.tokens ?? []} /></Text>);
        break;
      default:
        children.push(t.text ?? "");
    }
  }
  return <Text>{children}</Text>;
}

function Block({ token }: { token: Token }) {
  switch (token.type) {
    case "heading":
      return <Text bold color="white">{"#".repeat(token.depth)} <Inline tokens={token.tokens ?? []} /></Text>;
    case "paragraph":
      return <Text><Inline tokens={token.tokens ?? []} /></Text>;
    case "code":
      return <Box borderStyle="round" borderColor="gray" paddingX={1} marginY={1}><Text color="gray">{String(token.text).trimEnd()}</Text></Box>;
    case "blockquote":
      return <Box paddingLeft={1} borderStyle="single" borderColor="gray" flexDirection="column">{(token.tokens ?? []).map((t: Token, i: number) => <Block key={i} token={t} />)}</Box>;
    case "list": {
      const ordered = Boolean(token.ordered);
      return (
        <Box flexDirection="column">
          {(token.items ?? []).map((item: Token, i: number) => {
            const marker = ordered ? String(i + 1) + "." : "\u2022";
            const first = item.tokens?.[0];
            const rest = item.tokens?.slice(1) ?? [];
            return (
              <Box key={i} flexDirection="column">
                {first?.type === "paragraph" ? <Text>{marker} <Inline tokens={first.tokens ?? []} /></Text> : first ? <Block token={first} /> : null}
                {rest.map((t: Token, j: number) => <Block key={j} token={t} />)}
              </Box>
            );
          })}
        </Box>
      );
    }
    case "space":
      return <Text> </Text>;
    case "hr":
      return <Text color="gray">{"\u2500".repeat(20)}</Text>;
    default:
      return token.text ? <Text>{String(token.text)}</Text> : null;
  }
}

export function Markdown({ text }: { text: string }) {
  let tokens: Token[];
  try {
    tokens = marked.lexer(text) as Token[];
  } catch {
    return <Text>{text}</Text>;
  }
  return (
    <Box flexDirection="column">
      {tokens.map((t, i) => <Block key={i} token={t} />)}
    </Box>
  );
}