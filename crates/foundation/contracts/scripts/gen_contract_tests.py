#!/usr/bin/env python3
"""Generate Rust contract-fixture tests from the Go daemon contract suite.

Parses daemon/internal/contracts/*_test.go, extracts every schemaPath ->
fixture set (inline maps, xxxFixtures() helpers, single-fixture
ValidateRelative calls, []string fixture arrays, and file-backed testdata
fixtures), and emits one Rust integration-test file per Go file under
rs/contracts/tests. Fixture data lives in tests/common/data.rs; tests
reference it through common::data::<fn>() exactly like the Go tests call
their xxxFixtures() helpers.
"""

import os
import re

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
CONTRACTS_DIR = os.path.join(REPO_ROOT, "daemon", "internal", "contracts")
TESTDATA_DIR = os.path.join(CONTRACTS_DIR, "testdata")
OUT_DIR = os.path.join(REPO_ROOT, "rs", "contracts", "tests")

BT = chr(96)  # backtick


# ---------------------------------------------------------------------------
# Go source scanning
# ---------------------------------------------------------------------------

def skip_ws(src, i):
    while i < len(src) and src[i] in " \t\r\n":
        i += 1
    return i


def go_str_literal(src, i):
    """Parse a Go string literal starting at src[i]; returns (value, end)."""
    if src[i] == BT:
        j = src.index(BT, i + 1)
        return src[i + 1 : j], j + 1
    assert src[i] == '"', "not a string at %d: %r" % (i, src[i : i + 20])
    j = i + 1
    out = []
    simple = {
        "n": "\n", "t": "\t", "r": "\r", '"': '"', "\\": "\\", "/": "/",
        "b": "\b", "f": "\f", "a": "\a", "v": "\v", "0": "\0",
    }
    while j < len(src):
        c = src[j]
        if c == "\\":
            nxt = src[j + 1]
            if nxt in simple:
                out.append(simple[nxt]); j += 2
            elif nxt == "x":
                out.append(chr(int(src[j + 2 : j + 4], 16))); j += 4
            elif nxt == "u":
                out.append(chr(int(src[j + 2 : j + 6], 16))); j += 6
            elif nxt == "U":
                out.append(chr(int(src[j + 2 : j + 10], 16))); j += 10
            else:
                raise ValueError("unknown Go escape \\" + nxt)
        elif c == '"':
            return "".join(out), j + 1
        else:
            out.append(c); j += 1
    raise ValueError("unterminated Go string literal")


def skip_comment(src, i):
    """If src[i:] starts a // or /* comment, return index after it; else i."""
    if src[i : i + 2] == "//":
        j = src.find("\n", i)
        return j + 1 if j != -1 else len(src)
    if src[i : i + 2] == "/*":
        j = src.find("*/", i + 2)
        return j + 2 if j != -1 else len(src)
    return i



def stmt_start(body, i):
    """True if body[i] begins a Go statement: at source start, after ';' or
    '{', or after a newline (Go's automatic semicolon insertion)."""
    j = i - 1
    while j >= 0 and body[j] in " \t":
        j -= 1
    return j < 0 or body[j] in "\n;{"


def split_top_level(s, sep=","):
    """Split a Go expression list on sep at depth 0 (string/bracket aware)."""
    parts, cur, depth = [], "", 0
    j = 0
    while j < len(s):
        c = s[j]
        if c in '"' + BT:
            lit, j2 = go_str_literal(s, j)
            cur += s[j:j2]
            j = j2
            continue
        if c in "([{":
            depth += 1
        elif c in ")]}":
            depth -= 1
        elif c == sep and depth == 0:
            parts.append(cur)
            cur = ""
            j += 1
            continue
        cur += c
        j += 1
    parts.append(cur)
    return [p.strip() for p in parts]


def loop_validates(body, brace_idx):
    """True if the for-loop body starting at brace_idx calls ValidateRelative."""
    depth = 1
    j = brace_idx + 1
    while j < len(body):
        c = body[j]
        if c in '"' + BT:
            _, j = go_str_literal(body, j)
            continue
        if c == "/" and (body[j : j + 2] == "//" or body[j : j + 2] == "/*"):
            j = skip_comment(body, j)
            continue
        if c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                break
        j += 1
    loop_body = body[brace_idx + 1 : j]
    return "ValidateRelative(" in loop_body or "mustValidate(" in loop_body


def scan_expr(src, i, stops=",}]"):
    """Scan one complete Go expression; returns (expr, end) where end is the
    index just past the last consumed character. Stops at a top-level stop
    character (end points AT it), or right after a token at depth 0 whose
    successor is not a postfix continuation (end includes the token)."""
    depth = 0
    j = i

    def continuation(nxt):
        k = nxt
        while k < len(src) and src[k] in " \t":
            k += 1
        if k < len(src) and src[k] in ".[(+":
            return True
        # type-cast-like postfix: identifier immediately followed by '('
        # (e.g. []byte(...), map[string]string(...))
        if k < len(src) and (src[k].isalpha() or src[k] == "_"):
            kk = k
            while kk < len(src) and (src[kk].isalnum() or src[kk] == "_"):
                kk += 1
            while kk < len(src) and src[kk] in " \t":
                kk += 1
            return kk < len(src) and src[kk] in "({"
        return False

    while j < len(src):
        c = src[j]
        if c in '"' + BT:
            _, j = go_str_literal(src, j)
            if depth == 0:
                nxt = j
                while nxt < len(src) and src[nxt] in " \t":
                    nxt += 1
                if nxt < len(src) and src[nxt] in ".[(+":
                    continue
                if nxt < len(src) and src[nxt] == "," and "," not in stops:
                    continue
                return src[i:j], j
            continue
        if c == "/" and (src[j : j + 2] == "//" or src[j : j + 2] == "/*"):
            j = skip_comment(src, j)
            continue
        if c in "([{":
            depth += 1
        elif c in ")]}":
            if depth == 0:
                return src[i:j], j
            depth -= 1
            if depth == 0 and not continuation(j + 1):
                return src[i : j + 1], j + 1
        elif depth == 0 and c in stops:
            return src[i:j], j
        j += 1
    return src[i:j], j


def parse_map_literal(src, start):
    """Parse map[string]string{...} body starting at '{' (index start).
    Returns (entries[(key, expr)], end_after_brace)."""
    i = start + 1
    entries = []
    while True:
        i = skip_ws(src, i)
        if i >= len(src):
            raise ValueError("unterminated map literal")
        if src[i] == "}":
            return entries, i + 1
        if src[i] not in '"' + BT:
            raise ValueError("expected string key at %d: %r" % (i, src[i : i + 40]))
        key, i = go_str_literal(src, i)
        i = skip_ws(src, i)
        assert src[i] == ":", "expected ':' at %d" % i
        value, i = scan_expr(src, skip_ws(src, i + 1))
        entries.append((key, value.strip()))
        i = skip_ws(src, i)
        if i >= len(src):
            raise ValueError("unterminated map literal")
        if src[i] == ",":
            i += 1
        elif src[i] == "}":
            return entries, i + 1
        else:
            raise ValueError("unexpected %r at %d in map literal" % (src[i], i))


def parse_slice_literal(src, start):
    i = start + 1
    items = []
    while True:
        i = skip_ws(src, i)
        if src[i] == "}":
            return items, i + 1
        item, i = scan_expr(src, i)
        items.append(item.strip())
        i = skip_ws(src, i)
        if src[i] == ",":
            i += 1
        elif src[i] == "}":
            return items, i + 1


def find_funcs(src):
    out = []
    for m in re.finditer(r"^func\s+(\w+)\(([^)]*)\)\s*(?:map\[string\]string\s*)?\{", src, re.M):
        name, params, start = m.group(1), m.group(2), m.end()
        depth = 1
        j = start
        while j < len(src):
            c = src[j]
            if c in '"' + BT:
                _, j = go_str_literal(src, j)
                continue
            if c == "/" and (src[j : j + 2] == "//" or src[j : j + 2] == "/*"):
                j = skip_comment(src, j)
                continue
            if c == "{":
                depth += 1
            elif c == "}":
                depth -= 1
                if depth == 0:
                    out.append((name, params, start, j))
                    break
            j += 1
        else:
            raise ValueError("unbalanced func " + name)
    return out


# ---------------------------------------------------------------------------
# Expression resolution
# ---------------------------------------------------------------------------

class Resolver:
    def __init__(self):
        self.file_helpers = {}    # helper name -> list[(path, fixture)]
        self.assert_helpers = {}  # assert helper name -> list of helper names

    def resolve(self, expr, local_vars, context):
        e = expr.strip()
        if not e:
            raise ValueError("empty expression in " + context)
        # single string literal
        if e[0] in '"' + BT:
            try:
                value, end = go_str_literal(e, 0)
                if end == len(e):
                    return ("str", value)
            except ValueError:
                pass
        # wrappers
        m = re.fullmatch(r"(?:\[\]byte|string)\((.*)\)", e, re.S)
        if m:
            return self.resolve(m.group(1).strip(), local_vars, context)
        # inline map literal
        if e.startswith("map[string]string{"):
            entries, _ = parse_map_literal(e, len("map[string]string"))
            resolved = [(k, self.resolve_str(v, local_vars, context)) for k, v in entries]
            return ("map", resolved)
        # concatenation
        parts = self._split_concat(e)
        if len(parts) > 1:
            out = ""
            for part in parts:
                kind, val = self.resolve(part, local_vars, context)
                if kind != "str":
                    raise ValueError("concat of non-string: " + part)
                out += val
            return ("str", out)
        # identifier
        if re.fullmatch(r"[A-Za-z_]\w*", e):
            if e in local_vars:
                return local_vars[e]
            if e in self.file_helpers:
                return ("map", self.file_helpers[e])
            raise ValueError("unknown identifier " + e)
        # call
        m = re.fullmatch(r"(\w+)\((.*)\)", e, re.S)
        if m:
            fname, args = m.group(1), m.group(2).strip()
            if fname in self.file_helpers and args == "":
                return ("map", self.file_helpers[fname])
            if fname == "mustReadChannelManagementFixture":
                am = re.fullmatch(r"t\s*,\s*\w+\s*,\s*\"([^\"]*)\"", args)
                if am:
                    path = os.path.join(TESTDATA_DIR, "channel-management", am.group(1))
                    with open(path) as fh:
                        return ("str", fh.read())
                raise ValueError("unhandled mustRead args: " + args)
            if fname == "mustValidateFixtures":
                am = args.split(",", 2)
                if len(am) == 3:
                    return self.resolve(am[2].strip(), local_vars, context)
                raise ValueError("unhandled mustValidateFixtures args: " + args)
            raise ValueError("unknown call " + fname)
        # map index: X["key"]
        m = re.fullmatch(r"(.+?)\[\s*(\"[^\"]*\"|" + BT + r"[^" + BT + r"]*" + BT + r"|\w+)\s*\]", e, re.S)
        if m:
            base, key_expr = m.group(1).strip(), m.group(2)
            if key_expr[0] in '"' + BT:
                key, _ = go_str_literal(key_expr, 0)
            else:
                kind, val = self.resolve(key_expr, local_vars, context)
                if kind != "str":
                    raise ValueError("map index by non-string " + key_expr)
                key = val
            kind, entries = self.resolve(base, local_vars, context)
            if kind != "map":
                raise ValueError("indexing non-map " + base)
            for k, v in entries:
                if k == key:
                    return ("str", v) if isinstance(v, str) else v
            raise ValueError("key %r not found in %s" % (key, base))
        raise ValueError("cannot resolve expression %r in %s" % (e, context))

    def resolve_str(self, expr, local_vars, context):
        kind, val = self.resolve(expr, local_vars, context)
        if kind != "str":
            raise ValueError("expected string for %r in %s" % (expr, context))
        return val

    def _split_concat(self, e):
        parts, cur, depth = [], "", 0
        j = 0
        while j < len(e):
            c = e[j]
            if c in '"' + BT:
                lit, j2 = go_str_literal(e, j)
                cur += e[j:j2]
                j = j2
                continue
            if c in "([{":
                depth += 1
            elif c in ")]}":
                depth -= 1
            elif c == "+" and depth == 0:
                parts.append(cur)
                cur = ""
                j += 1
                continue
            cur += c
            j += 1
        parts.append(cur)
        return [p.strip() for p in parts if p.strip()]


# ---------------------------------------------------------------------------
# Function analysis
# ---------------------------------------------------------------------------

def collect_function_data(src, fname, fstart, fend, resolver):
    """For a test function, return a list of ops:
       ('maps', helper_name) | ('ok', path, fixture) | ('neg', path, fixture)"""
    body = src[fstart:fend]
    local_vars = {}
    ops = []

    def add_map(entries):
        return [(k, resolver.resolve_str(v, local_vars, fname)) for k, v in entries]

    i = 0
    while i < len(body):
        c = body[i]
        if c in '"' + BT:
            _, i = go_str_literal(body, i)
            continue
        if c == "/" and (body[i : i + 2] == "//" or body[i : i + 2] == "/*"):
            i = skip_comment(body, i)
            continue
        if c == "m" and body.startswith("map[string]string{", i):
            entries, end = parse_map_literal(body, i + len("map[string]string"))
            add_map(entries)
            i = end
            continue
        if c == "m" and body.startswith("mustValidateFixtures", i):
            j = body.find("(", i)
            args, end = scan_expr(body, j + 1, stops=")")
            parts = split_top_level(args)
            if len(parts) >= 3:
                target = parts[2]
                mh = re.fullmatch(r"(\w+)\(\)", target)
                if mh and mh.group(1) in resolver.file_helpers:
                    ops.append(("maps", mh.group(1), None))
                else:
                    try:
                        kind, val = resolver.resolve(target, local_vars, fname)
                        if kind == "map":
                            for k, v in val:
                                ops.append(("ok", k, v))
                    except ValueError:
                        pass
            i = end
            continue
        if c == "v" and body.startswith("validator.ValidateRelative", i):
            j = body.find("(", i)
            args, end = scan_expr(body, j + 1, stops=")")
            am = [p.strip() for p in args.split(",", 1)]
            if len(am) == 2:
                try:
                    pk, path = resolver.resolve(am[0], local_vars, fname)
                    if pk == "str":
                        ahead = body[end : end + 120]
                        neg = re.search(r"err\s*==\s*nil", ahead) is not None
                        dk, doc = resolver.resolve(am[1], local_vars, fname)
                        if dk == "str":
                            ops.append(("neg" if neg else "ok", path, doc))
                except ValueError:
                    pass
            i = end
            continue
        if c == "a":
            m = re.match(r"(assert[A-Za-z_]\w*)\(", body[i:])
            if m and m.group(1) in resolver.assert_helpers:
                j = body.find("(", i)
                _, end = scan_expr(body, j + 1, stops=")")
                for helper in resolver.assert_helpers[m.group(1)]:
                    ops.append(("maps", helper, None))
                i = end
                continue
        m = re.match(r"(\w+)\s*:=\s*", body[i:])
        if m and m.group(1) != "err" and stmt_start(body, i):
            varname = m.group(1)
            rest = i + m.end()
            if body[rest:].startswith("map[string]string{"):
                entries, end = parse_map_literal(body, rest + len("map[string]string"))
                local_vars[varname] = ("map", add_map(entries))
                i = end
                continue
            if body[rest:].startswith("[]string{"):
                items, end = parse_slice_literal(body, rest + len("[]string"))
                local_vars[varname] = ("slice", items)
                i = end
                continue
            try:
                val, end = scan_expr(body, rest)
                kind, resolved = resolver.resolve(val, local_vars, fname)
                if kind in ("str", "map"):
                    local_vars[varname] = (kind, resolved)
                    i = end
                    continue
            except ValueError:
                pass
            val, end = scan_expr(body, rest)
            i = end
            continue
        i += 1

    # loop over a fixture helper: for name, fixture := range helper()
    for m in re.finditer(r"for\s+(schemaPath|schema|name)\s*,\s*fixture\s*:=\s*range\s+(\w+)\(\)\s*\{", body):
        helper = m.group(2)
        if helper in resolver.file_helpers and loop_validates(body, m.end() - 1):
            for k, v in resolver.file_helpers[helper]:
                ops.append(("ok", k, v))
    # loop over a local map variable
    for m in re.finditer(r"for\s+(schemaPath|schema|name)\s*,\s*fixture\s*:=\s*range\s+(\w+)\s*\{", body):
        mapvar = m.group(2)
        if mapvar in local_vars and local_vars[mapvar][0] == "map" and loop_validates(body, m.end() - 1):
            for k, v in local_vars[mapvar][1]:
                ops.append(("ok", k, v))
    # loop directly over an inline map literal
    m = re.search(r"for\s+(schemaPath|schema|name)\s*,\s*fixture\s*:=\s*range\s+map\[string\]string\{", body)
    if m:
        idx = body.find("map[string]string{", m.start())
        entries, map_end = parse_map_literal(body, idx + len("map[string]string"))
        j = map_end
        while j < len(body) and body[j] in " \t\r\n":
            j += 1
        if j < len(body) and body[j] == "{" and loop_validates(body, j):
            for k, v in add_map(entries):
                ops.append(("ok", k, v))
    # []string fixture arrays validated against one schema path
    for varname, val in list(local_vars.items()):
        if val[0] != "slice":
            continue
        lm = re.search(r"for\s*_,\s*(\w+)\s*:=\s*range\s+" + re.escape(varname) + r"\s*\{", body)
        if not lm:
            continue
        loopvar = lm.group(1)
        vm = re.search(
            r"ValidateRelative\(\s*(\"[^\"]*\"|" + BT + r"[^" + BT + r"]*" + BT + r")\s*,\s*\[\]byte\(\s*"
            + re.escape(loopvar) + r"\s*\)\s*\)",
            body,
        )
        if vm:
            path, _ = go_str_literal(vm.group(1), 0)
            for item in val[1]:
                try:
                    ops.append(("ok", path, resolver.resolve_str(item, local_vars, fname)))
                except ValueError:
                    pass

    # dedupe while preserving order
    seen = set()
    out = []
    for op in ops:
        key = op if op[0] != "maps" else ("maps", op[1])
        if key not in seen:
            seen.add(key)
            out.append(op)
    return out


# ---------------------------------------------------------------------------
# Rust emission
# ---------------------------------------------------------------------------

def to_snake(name):
    return re.sub(r"(?<!^)(?=[A-Z])", "_", name).lower()


def rust_str(value):
    assert '"##' not in value, "fixture contains quote-hash sequence"
    return 'r##"' + value + '"##'


def emit_test_file(go_stem, cases):
    lines = [
        "//! Ported from daemon/internal/contracts/" + go_stem + " (wave 8 contract parity).",
        "//!",
        "//! Each test mirrors the corresponding Go test function: the same",
        "//! schemaPath -> fixture set is validated through",
        "//! Validator::validate_relative (Go ValidateRelative).",
        "",
        "mod common;",
        "",
        "use common::{schema_root_dir, validate_fixtures};",
        "use kura_contracts::Validator;",
        "",
    ]
    if any(op[0] == "ok" for _, ops in cases for op in ops):
        lines[8] = lines[8].replace(
            "use common::{schema_root_dir, validate_fixtures};",
            "use common::{schema_root_dir, validate_fixtures, Fixture};",
        )
    for go_name, ops in cases:
        lines.append("#[test]")
        lines.append("fn " + to_snake(go_name) + "() {")
        lines.append("    let validator = Validator::new(schema_root_dir());")
        map_refs = [to_snake(op[1]) for op in ops if op[0] == "maps"]
        ok_ops = [op for op in ops if op[0] == "ok"]
        neg_ops = [op for op in ops if op[0] == "neg"]
        emitted = False
        if map_refs:
            refs = ", ".join("common::data::" + r + "()" for r in sorted(set(map_refs)))
            lines.append("    validate_fixtures(&validator, &[" + refs + "].concat());")
            emitted = True
        if ok_ops:
            if not emitted:
                lines.append("    let fixtures: &[Fixture] = &[")
                for _, path, fixture in ok_ops:
                    lines.append("        (" + rust_str(path) + ", " + rust_str(fixture) + "),")
                lines.append("    ];")
                lines.append("    validate_fixtures(&validator, fixtures);")
            else:
                lines.append("    let extra: &[Fixture] = &[")
                for _, path, fixture in ok_ops:
                    lines.append("        (" + rust_str(path) + ", " + rust_str(fixture) + "),")
                lines.append("    ];")
                lines.append("    validate_fixtures(&validator, extra);")
            emitted = True
        if neg_ops:
            for _, path, fixture in neg_ops:
                lines.append(
                    "    let err = validator.validate_relative(" + rust_str(path) + ", "
                    + rust_str(fixture) + ".as_bytes());"
                )
                lines.append("    assert!(err.is_err(), \"expected fixture to fail schema validation\");")
        lines.append("}")
        lines.append("")
    return "\n".join(lines)


def main():
    resolver = Resolver()
    files = sorted(f for f in os.listdir(CONTRACTS_DIR) if f.endswith("_test.go"))
    file_funcs = {}
    for f in files:
        src = open(os.path.join(CONTRACTS_DIR, f)).read()
        file_funcs[f] = (src, find_funcs(src))

    # pass 1a: fixture helper functions
    for f in files:
        src, funcs = file_funcs[f]
        for name, params, start, end in funcs:
            if params != "" or not re.fullmatch(r"\w+Fixtures\w*", name):
                continue
            body = src[start:end]
            local_vars = {}
            entries = []
            i = 0
            while i < len(body):
                c = body[i]
                if c in '"' + BT:
                    _, i = go_str_literal(body, i)
                    continue
                if c == "/" and (body[i : i + 2] == "//" or body[i : i + 2] == "/*"):
                    i = skip_comment(body, i)
                    continue
                if c == "m" and body.startswith("map[string]string{", i):
                    ents, end = parse_map_literal(body, i + len("map[string]string"))
                    for k, v in ents:
                        entries.append((k, resolver.resolve_str(v, local_vars, name)))
                    i = end
                    continue
                m = re.match(r"(\w+)\s*:=\s*", body[i:])
                if m and m.group(1) != "err" and stmt_start(body, i):
                    rest = i + m.end()
                    if body[rest:].startswith("map[string]string{"):
                        ents, end = parse_map_literal(body, rest + len("map[string]string"))
                        resolved = [(k, resolver.resolve_str(v, local_vars, name)) for k, v in ents]
                        local_vars[m.group(1)] = ("map", resolved)
                        entries.extend(resolved)
                        i = end
                        continue
                    try:
                        val, end = scan_expr(body, rest)
                        kind, resolved = resolver.resolve(val, local_vars, name)
                        if kind in ("str", "map"):
                            local_vars[m.group(1)] = (kind, resolved)
                            i = end
                            continue
                    except ValueError:
                        pass
                    val, end = scan_expr(body, rest)
                    i = end
                    continue
                i += 1
            if entries:
                resolver.file_helpers[name] = entries

    # pass 1b: assert helpers (mustValidateFixtures(t, validator, <helper>()))
    for f in files:
        src, funcs = file_funcs[f]
        for name, params, start, end in funcs:
            if not name.startswith("assert"):
                continue
            body = src[start:end]
            sources = []
            for m in re.finditer(r"mustValidateFixtures\(t\s*,\s*validator\s*,\s*(\w+)\(\)\)", body):
                helper = m.group(1)
                if helper in resolver.file_helpers:
                    sources.append(helper)
            if sources:
                resolver.assert_helpers[name] = sources

    # pass 2: test functions
    generated = {}
    for f in files:
        src, funcs = file_funcs[f]
        for name, params, start, end in funcs:
            if not name.startswith("Test"):
                continue
            ops = collect_function_data(src, name, start, end, resolver)
            if os.environ.get("GEN_DEBUG"):
                print("  test %s: %d ops" % (name, len(ops)), [op[0] for op in ops][:6])
            if ops:
                generated.setdefault(f[:-8], []).append((name, ops))

    # write common modules
    os.makedirs(os.path.join(OUT_DIR, "common"), exist_ok=True)
    mod_lines = [
        "//! Shared helpers for the ported daemon contract-fixture tests (wave 8).",
        "//!",
        "//! Mirrors the Go helpers in daemon/internal/contracts: schemaRootDir",
        "//! and mustValidateFixtures, plus the shared data module holding the",
        "//! fixture maps that several Go test files reference.",
        "",
        "use std::path::PathBuf;",
        "",
        "pub mod data;",
        "",
        "pub type Fixture = (&'static str, &'static str);",
        "",
        "/// Mirrors Go's schemaRootDir: the repository root containing the",
        "/// schemas/ tree.",
        "pub fn schema_root_dir() -> PathBuf {",
        "    let root = PathBuf::from(env!(\"CARGO_MANIFEST_DIR\")).join(\"..\").join(\"..\");",
        "    assert!(",
        "        root.join(\"schemas\").is_dir(),",
        "        \"schemas/ directory not found under {}\",",
        "        root.display()",
        "    );",
        "    root",
        "}",
        "",
        "/// Mirrors Go's mustValidateFixtures: every schemaPath -> fixture pair must",
        "/// validate cleanly.",
        "pub fn validate_fixtures(validator: &kura_contracts::Validator, fixtures: &[Fixture]) {",
        "    for (schema_path, fixture) in fixtures {",
        "        validator",
        "            .validate_relative(schema_path, fixture.as_bytes())",
        "            .unwrap_or_else(|err| panic!(\"ValidateRelative({schema_path}): {err}\"));",
        "    }",
        "}",
        "",
    ]
    with open(os.path.join(OUT_DIR, "common", "mod.rs"), "w") as fh:
        fh.write("\n".join(mod_lines))

    data_lines = [
        "#![allow(dead_code)]",
        "//! Fixture data shared across the ported contract tests.",
        "//!",
        "//! Each function mirrors a func xxxFixtures() map[string]string helper in",
        "//! daemon/internal/contracts and is referenced by the test files exactly as",
        "//! the Go tests call their helpers.",
        "",
        "use super::Fixture;",
        "",
    ]
    for hname in sorted(resolver.file_helpers):
        data_lines.append("pub fn " + to_snake(hname) + "() -> &'static [Fixture] {")
        data_lines.append("    &[")
        for path, fixture in resolver.file_helpers[hname]:
            data_lines.append("        (" + rust_str(path) + ", " + rust_str(fixture) + "),")
        data_lines.append("    ]")
        data_lines.append("}")
        data_lines.append("")
    with open(os.path.join(OUT_DIR, "common", "data.rs"), "w") as fh:
        fh.write("\n".join(data_lines))

    total_tests = 0
    total_fixtures = 0
    all_schemas = set()
    for go_stem in sorted(generated):
        cases = generated[go_stem]
        text = emit_test_file(go_stem + "_test.go", cases)
        with open(os.path.join(OUT_DIR, go_stem + ".rs"), "w") as fh:
            fh.write(text)
        n_tests = len(cases)
        n_fix = sum(len([op for op in ops if op[0] in ("ok", "neg")]) for _, ops in cases)
        total_tests += n_tests
        total_fixtures += n_fix
        for _, ops in cases:
            for op in ops:
                if op[0] in ("ok", "neg"):
                    all_schemas.add(op[1])
        print("wrote %s.rs: %d tests, %d inline fixtures" % (go_stem, n_tests, n_fix))

    helper_fix = sum(len(v) for v in resolver.file_helpers.values())
    for v in resolver.file_helpers.values():
        for p, _ in v:
            all_schemas.add(p)
    print("")
    print("TOTAL: %d test functions, %d inline fixtures, %d helper-map fixtures, %d unique schemas referenced"
          % (total_tests, total_fixtures, helper_fix, len(all_schemas)))


if __name__ == "__main__":
    main()