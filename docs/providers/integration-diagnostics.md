# Integration Diagnostics

Roadmap 42 adds tenant-scoped integration diagnostic state for provider authorization,
tenant approval, scopes, token freshness, provider availability, network failures, retry
safety, and unsupported-domain projections.

Feishu/Lark is the full proof domain for this phase. Other domains must expose either a
limited structured diagnostic result or a deliberate unsupported diagnostic
classification.

Diagnostic evidence must be redacted before persistence or display. If redaction cannot
be proven, diagnostics fail closed with `redaction_failed_closed` and emit audit
evidence.
