// DopeAgent official site: landing page + full documentation. Docs are
// markdown, rendered client-side; the architecture chapter imports the
// repository's real design doc so the site can never drift from it.

import { Link, NavLink, Navigate, Route, Routes, useLocation } from "react-router-dom";
import { useEffect } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";

import architectureDoc from "../../docs/harness/plugin-architecture.md?raw";
import apiDoc from "./content/api.md?raw";
import channelsDoc from "./content/channels.md?raw";
import configurationDoc from "./content/configuration.md?raw";
import contextSessionDoc from "./content/context-session.md?raw";
import externalPluginsDoc from "./content/external-plugins.md?raw";
import gettingStartedDoc from "./content/getting-started.md?raw";
import memoryDoc from "./content/memory.md?raw";
import pluginsDoc from "./content/plugins.md?raw";
import skillsImprovementDoc from "./content/skills-improvement.md?raw";
import usageDoc from "./content/usage.md?raw";

const REPO = "https://github.com/dopejs/dope-agent";

type DocPage = { slug: string; title: string; body: string };

const DOCS: DocPage[] = [
  { slug: "getting-started", title: "Getting Started", body: gettingStartedDoc },
  { slug: "usage", title: "Usage", body: usageDoc },
  { slug: "configuration", title: "Configuration", body: configurationDoc },
  { slug: "plugins", title: "Plugins", body: pluginsDoc },
  { slug: "external-plugins", title: "External Plugins", body: externalPluginsDoc },
  { slug: "memory", title: "Memory", body: memoryDoc },
  { slug: "context-session", title: "Context & Session", body: contextSessionDoc },
  { slug: "skills-improvement", title: "Skills & Self-Improvement", body: skillsImprovementDoc },
  { slug: "channels", title: "Channels", body: channelsDoc },
  { slug: "api", title: "API Reference", body: apiDoc },
  { slug: "architecture", title: "Architecture (design doc)", body: architectureDoc },
];

function ScrollToTop() {
  const { pathname } = useLocation();
  useEffect(() => window.scrollTo(0, 0), [pathname]);
  return null;
}

function Header() {
  return (
    <header className="header">
      <Link to="/" className="brand">
        <span className="brand__mark">⬡</span> DopeAgent
      </Link>
      <nav className="header__nav">
        <NavLink to="/docs/getting-started">Docs</NavLink>
        <a href={`${REPO}/releases`}>Releases</a>
        <a href={REPO}>GitHub</a>
      </nav>
    </header>
  );
}

const FEATURES: Array<{ title: string; body: string; to: string }> = [
  {
    title: "Everything is a plugin",
    body:
      "31+ builtin plugins over a small trust-boundary kernel. Disable, configure, or replace any of them from one profile file — dependencies resolve transitively and the assembly report tells you exactly what is running and why.",
    to: "/docs/plugins",
  },
  {
    title: "Layered memory (L0–L3)",
    body:
      "Episodes → atoms → scenarios → personas, every layer attributable and reversible. Five capture paths write automatically; an LLM consolidator distills upward; invented citations are discarded.",
    to: "/docs/memory",
  },
  {
    title: "A context engine that cites",
    body:
      "Memory bootstrap under budget, BM25+vector+RRF recall, symbolic compression of oversized content — every injected line carries its citation, and every decision lands in an inspectable AssemblyRecord.",
    to: "/docs/context-session",
  },
  {
    title: "Sessions that never forget",
    body:
      "Frame-preserving windows: long personal sessions (48k) and one-context-per-IM-thread (16k). Evicted spans are captured to memory first and the elision marker cites them — the model can always drill back.",
    to: "/docs/context-session",
  },
  {
    title: "External plugins, any language",
    body:
      "Drop a manifest + process under plugins/. Attach to the hook waterfall, veto turns, rewrite context, or serve whole seams (bring your own embedding model) over a line-JSON stdio protocol.",
    to: "/docs/external-plugins",
  },
  {
    title: "Governed autonomy",
    body:
      "The agent proposes skills and config improvements; the operator approves. Evidence required, rate-bounded, rollback recorded before anything applies, full event audit chain.",
    to: "/docs/skills-improvement",
  },
];

function Landing() {
  return (
    <main className="landing">
      <section className="hero">
        <p className="hero__kicker">model-visible = logged</p>
        <h1>
          A <em>personal agent OS</em> you can open up and read.
        </h1>
        <p className="hero__sub">
          A Rust daemon that owns runtime, memory, context, and policy — with a
          plugin architecture where session management, retrieval, and even the
          embedding model are swappable parts. Thin clients: terminal, web,
          and your IM channels.
        </p>
        <div className="hero__actions">
          <Link className="button button--primary" to="/docs/getting-started">
            Get started
          </Link>
          <a className="button" href={`${REPO}/releases`}>
            Download v0.1.0
          </a>
        </div>
        <pre className="hero__terminal">
          <code>
            {`$ dope
[dope] listening on http://127.0.0.1:19191

$ curl -s localhost:19191/v1/chat/query \\
    -d '{"query":"hello","provider":"echo"}'`}
          </code>
        </pre>
      </section>

      <section className="features">
        {FEATURES.map((feature) => (
          <Link key={feature.title} to={feature.to} className="feature">
            <h3>{feature.title}</h3>
            <p>{feature.body}</p>
          </Link>
        ))}
      </section>

      <section className="strip">
        <div>
          <h2>Built to be audited</h2>
          <p>
            Every mutation is an event. Every context assembly is a record.
            Every memory cites its source. Every agent-authored change waits
            for your approval and carries its rollback. The daemon&apos;s
            entire composition is one <code>GET /v1/plugins</code> away.
          </p>
        </div>
        <div>
          <h2>Channels included</h2>
          <p>
            Discord, Telegram, Slack, and Matrix connectors run in-daemon —
            off by default, allowlist-gated, one context per thread. Feishu
            calendar &amp; mail ride the integrations plane.
          </p>
        </div>
      </section>
      <Footer />
    </main>
  );
}

function DocsLayout() {
  const { pathname } = useLocation();
  const current = DOCS.find((doc) => pathname.endsWith(`/${doc.slug}`));
  return (
    <div className="docs">
      <aside className="docs__sidebar">
        <p className="docs__sidebar-title">Documentation</p>
        <nav>
          {DOCS.map((doc) => (
            <NavLink key={doc.slug} to={`/docs/${doc.slug}`}>
              {doc.title}
            </NavLink>
          ))}
        </nav>
      </aside>
      <article className="docs__body">
        {current ? (
          <Markdown remarkPlugins={[remarkGfm]}>{current.body}</Markdown>
        ) : (
          <Navigate to="/docs/getting-started" replace />
        )}
        <Footer />
      </article>
    </div>
  );
}

function Footer() {
  return (
    <footer className="footer">
      <span>DopeAgent — a personal agent OS.</span>
      <span>
        <a href={REPO}>GitHub</a> · <a href={`${REPO}/releases`}>Releases</a> ·{" "}
        <a href={`${REPO}/issues`}>Issues</a>
      </span>
    </footer>
  );
}

export default function App() {
  return (
    <>
      <ScrollToTop />
      <Header />
      <Routes>
        <Route path="/" element={<Landing />} />
        <Route path="/docs" element={<DocsLayout />} />
        <Route path="/docs/:slug" element={<DocsLayout />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </>
  );
}
