# Operator Shell And Onboarding

Status: proposed

Authority: This document is the authoritative upstream spec for the product shell required to operate a production personal agent rather than a developer daemon.

Primary source documents:
- `docs/product/personal-agent-non-knowledge-roadmap-split.md`
- `docs/runtime/daemon-api-and-event-model.md`
- `docs/product/feature-phasing.md`

## Background

Even with strong runtime capabilities, the product remains developer-shaped unless setup, inspection, approvals, health, and history are available through a coherent operator shell.

## Goal

Add the minimum onboarding and operator-control surface required to configure, inspect, and trust the personal-agent product.

## Fixed Decisions

- this is not visual polish only; it is an operator-control roadmap
- onboarding, approvals, diagnostics, and history are in scope
- the shell must consume daemon truth rather than invent client-side state

## In Scope

- first-run onboarding
- connector, integration, and capability readiness views
- approval inbox
- schedule, workflow, and delivery inspection
- health and diagnostics views

## Out Of Scope

- marketing site work
- collaborative multi-user admin console
- rich design-system expansion unrelated to operator tasks

## User Stories

- As a user, I can set up the daemon and understand what still needs configuration.
- As an operator, I can inspect approvals, schedules, workflows, and delivery outcomes in one shell.
- As an operator, I can debug why a background task did not run or did not deliver.

## Functional Requirements

- the product MUST provide a coherent onboarding path for auth, integration readiness, and first useful action
- the shell MUST surface approvals, schedules, workflows, and delivery truth without requiring raw API use
- the shell MUST provide health and failure diagnostics for integrations, schedules, and computer-use paths

## Verification Expectations

- client tests for critical shell flows
- API-to-UI contract coverage where practical
- one manual onboarding acceptance path in `DOPE_ENV=test`

## Definition Of Done

- a non-developer operator can set up and inspect the personal-agent system without dropping to raw daemon routes for basic control

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/017-operator-shell-and-onboarding.md 完成 phase 32 的工作`
