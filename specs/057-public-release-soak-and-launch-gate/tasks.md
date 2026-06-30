# Tasks: Public Release Soak And Launch Gate

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Roadmap**: 72

- [X] T001 [Setup] Reuse opsreadiness RealAccountSmokeStatus + ValidateRealAccountSmoke + smoke builders.
- [X] T002 [Foundational] RequiredLaunchWorkloads + LaunchGateEvidence/WorkloadEvidence/LaunchDecision + gate statement.
- [X] T003 [US1] ValidateLaunchGate: required-workload coverage, >=3 channels, calendar+mail providers, soak/support/redaction; ship vs no-ship + entry-gate flag.
- [X] T004 [US2] specific no-ship reasons per missing/failed input; skip-needs-reason.
- [X] T005 [P] tests: ship + missing/failed workload + <3 channels + missing provider + soak/support/redaction + skip-reason.
- [X] T006 [API] POST /v1/release/launch-gate validator endpoint; RealAccountSmokeStatus JSON tags.
- [X] T007 [Polish] launch-gate decision schema + contract test; docs non-knowledge-parity marker; verify build/vet/test.

## Notes
Validator is the codified gate; the hosted soak + real-account runs (operator-run) feed the
evidence index. Context/knowledge/memory remain out of scope and are gated on this evidence.
