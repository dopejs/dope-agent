# Skill Registry And Prompt Support

## Purpose

This roadmap adds first-class daemon support for skills.

The target is not only to store a few local skill files. The target is to make skill loading and explicit skill usage part of the daemon contract.

## Supported Roots

The daemon supports two agent roots.

### Dope-Managed Root

This is the active Dope data dir for the running environment.

Expected files:

- `<dataDir>/AGENTS.md`
- `<dataDir>/skills/<skill-name>/SKILL.md`
- bundled files under each skill directory

### Home Root

This is the user-level agent root.

Expected files:

- `~/.agents/AGENTS.md` when present
- `~/.agents/skills/<skill-name>/SKILL.md`
- bundled files under each skill directory

## Supported Objects

### Skill

A skill is defined by a directory containing:

- required `SKILL.md`
- optional bundled files under the same skill root

The daemon should load:

- parsed name
- parsed description
- raw instruction body
- source root
- skill root path
- bundled file inventory

### Overlay

An overlay is an operator-authored markdown file that affects daemon-side prompt assembly.

For this roadmap, the supported overlays are:

- `<dataDir>/AGENTS.md`
- `~/.agents/AGENTS.md`

## Precedence

The registry must define precedence clearly.

For name resolution:

- `dataDir` skills shadow home skills when the same skill name exists in both places

For overlay application:

- home overlay is applied first
- `dataDir` overlay is applied second

This lets Dope-managed environment instructions narrow or override home-level defaults.

## Chat Behavior

Chat requests should remain explicit.

This roadmap does **not** add automatic skill inference.

Instead, the chat contract should accept a list of skill identifiers or names. The daemon then:

1. resolves the requested skills from the registry
2. loads supported overlays
3. compiles overlays and skill instructions into system messages
4. appends the user query as the user message
5. dispatches through the existing provider plane

## API Surface

The daemon should expose:

- `GET /v1/skills`
- `GET /v1/skills/{skillId}`
- `POST /v1/skills/reload`

These APIs should let operators inspect:

- loaded skills
- source root
- skill metadata
- bundled file inventory
- loaded overlays

## Explicitly Out Of Scope

- automatic skill triggering
- execution of bundled scripts
- reference-file auto-loading based on task content
- plugin marketplace behavior

## Completion Standard

This roadmap is complete only when:

- the daemon can load skills from `dataDir` and `~/.agents`
- the daemon can inspect those skills through API
- chat can explicitly apply selected skills
- `AGENTS.md` overlays from supported roots participate in prompt assembly
- contracts and tests prove the behavior end to end
