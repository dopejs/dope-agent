# Sandbox Backend Comparison

## Purpose

This document narrows the first backend choice for the sandbox execution plane.

The design target is a multi-backend substrate, but `Roadmap 16` should implement only one backend.

## Candidate Backends

### 1. Subprocess

Strengths:

- lowest implementation cost
- easiest to wire into current daemon
- best fit for managed CLI providers and local tools
- easiest to test in the current repository

Weaknesses:

- weakest isolation by default
- network controls are harder to hard-enforce
- filesystem controls begin as policy checks rather than full OS isolation

### 2. Docker

Strengths:

- stronger isolation boundary
- clearer filesystem and network control
- easier future path for reproducible tool environments

Weaknesses:

- larger operator burden
- daemon becomes dependent on docker availability and lifecycle
- test and prod local workflows become heavier

### 3. SSH

Strengths:

- useful for remote execution and bring-your-own-host models
- good long-term substrate for server or cluster execution

Weaknesses:

- not appropriate as the first backend
- significantly more operational coupling
- identity, credential, and host lifecycle become part of roadmap 16 by accident

### 4. Remote Managed Backend

Strengths:

- eventual path for cloud or worker-based execution
- best long-term separation of control plane and execution plane

Weaknesses:

- much too large for the first sandbox roadmap
- implies queueing, remote logs, and transport reliability work immediately

## Recommended First Backend

The recommended first backend is:

- `subprocess`

Reason:

- it is enough to validate the control-plane model
- it immediately improves provider bridges and future local tools
- it preserves the ability to add docker or remote backends later without wasting this roadmap

## Required Safeguards For Subprocess

The subprocess backend is acceptable only if Roadmap 16 includes:

- cwd control
- env filtering
- timeout and cancellation
- explicit filesystem policy checks
- explicit network policy declaration
- audit trail and decision recording

Without those, the subprocess backend becomes an ad hoc shell wrapper, which is not acceptable.
