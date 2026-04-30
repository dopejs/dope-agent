# Hosted Operational Profile Evidence

Hosted release-readiness evidence must come from a stable always-on test host
or VPS. Developer laptops are acceptable for targeted validation only; sleep,
power, and network behavior make laptop-only evidence insufficient for Roadmap
43 release readiness.

Record the host class, run identity, commit or version, daemon address, data
directory, artifact root, and any unsupported observations with every hosted
profile run.

Acceptable Roadmap 43 release evidence host classes:

- stable always-on test host
- VPS or cloud VM with sleep disabled and stable networking
- long-running CI host with persistent workspace and artifact retention

Developer laptops may be used for targeted validation of `provision`, `status`,
`health`, and local evidence generation, but they do not satisfy stable-host
release readiness. Laptop sleep, power changes, VPN changes, OS updates, and
Wi-Fi instability must be recorded as host-class limitations when used for
targeted checks.
