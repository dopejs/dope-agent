# Production Operations Baseline

Roadmap 39 validates a tenant-scoped single-node production baseline. The
default validation environment is `~/.dope-test` on `127.0.0.1:19192`; live
connectors and production user data require explicit operator opt-in.

Operator artifacts:

- [Production install](./production-install.md)
- [Production upgrade](./production-upgrade.md)
- [Backup and restore](./backup-restore.md)
- [Release readiness](./release-readiness.md)
- [Tenant migration rollback](./tenant-migration-rollback.md)
- [Production soak harness](../harness/production-soak.md)
- [Real-account smoke policy](../providers/real-account-smoke.md)

Multi-node managed service rollout, clustering, and distributed failover are
outside the Roadmap 39 baseline.
