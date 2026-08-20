# Quickstart: Mail Integration

## Goal

Verify in `KURA_ENV=test` that the daemon can:

- project a mailbox account from the shared integration substrate
- inspect thread, message, and draft state without send side effects
- create and update drafts truthfully
- send both direct messages and existing drafts while preserving explicit send-path truth
- reply and forward with truthful draft-only versus sent outcomes
- block final send when required attachment references are unresolved
- run background mail work through the existing workflow and delivery planes

## Prerequisites

- local test daemon only; do not use `~/.kura`
- authenticated local pairing or an existing bearer token
- no production connectors or live mail credentials are required
- the repo-owned fake mail backend is enabled through the fake integration path
- a `test_sink` delivery target exists if you want to validate background delivery reuse
- for a fully local background walkthrough, a workspace-local executable skill is enough;
  no external LLM provider is required

## Suggested Verification Flow

1. Start the daemon in the test environment.

```bash
make daemon-run-test
```

2. Register a fake mail integration and mark it healthy.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "integrationId": "mail-fake-primary",
    "domainKind": "mail",
    "displayName": "Mail Fake Primary",
    "backendKind": "fake_local",
    "backendRefId": "fake-mail-primary",
    "backendDisplayName": "Fake Mail Primary",
    "accountBinding": {
      "accountKey": "alice@example.com",
      "accountLabel": "Alice Mail"
    },
    "canonicalDefault": true
  }' \
  http://127.0.0.1:19192/v1/integrations
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "readinessStatus": "healthy",
    "authState": "authorized",
    "healthState": "healthy",
    "secretResolution": "resolved"
  }' \
  http://127.0.0.1:19192/v1/integrations/mail-fake-primary/readiness
```

Expected outcome after implementation:

- the integration is the canonical default for the fake mailbox
- readiness remains visible through `/v1/integrations`

3. Inspect the mailbox account projection.

```bash
curl -sS \
  -H "Authorization: Bearer $KURA_TOKEN" \
  http://127.0.0.1:19192/v1/mail/accounts
```

Expected outcome after implementation:

- the response includes `mail-fake-primary`
- the account projection shows mailbox identity and capability flags
- the chosen integration remains operator-visible

4. Inspect thread, message, and draft state without mutating send state.

```bash
curl -sS \
  -H "Authorization: Bearer $KURA_TOKEN" \
  "http://127.0.0.1:19192/v1/mail/threads?integrationId=mail-fake-primary"
```

```bash
curl -sS \
  -H "Authorization: Bearer $KURA_TOKEN" \
  "http://127.0.0.1:19192/v1/mail/messages/$MESSAGE_ID?integrationId=mail-fake-primary"
```

```bash
curl -sS \
  -H "Authorization: Bearer $KURA_TOKEN" \
  "http://127.0.0.1:19192/v1/mail/drafts?integrationId=mail-fake-primary"
```

Expected outcome after implementation:

- the responses return thread, message, and draft resources plus linked operation truth
- no sent-message side effect is recorded as part of inspection

5. Create and update one draft.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "integrationId": "mail-fake-primary",
    "composeMode": "new_message",
    "to": ["bob@example.com"],
    "subject": "Phase 30 draft",
    "body": "Draft created from quickstart."
  }' \
  http://127.0.0.1:19192/v1/mail/drafts
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "body": "Updated draft body."
  }' \
  http://127.0.0.1:19192/v1/mail/drafts/$DRAFT_ID/update
```

Expected outcome after implementation:

- the draft keeps a stable `draftId`
- the responses distinguish `create_draft` from `update_draft`
- operator-visible history records draft-only truth rather than a sent outcome

6. Send one direct new message and one existing draft.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "integrationId": "mail-fake-primary",
    "to": ["bob@example.com"],
    "subject": "Phase 30 direct send",
    "body": "Direct send from quickstart."
  }' \
  http://127.0.0.1:19192/v1/mail/messages/send
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}' \
  http://127.0.0.1:19192/v1/mail/drafts/$DRAFT_ID/send
```

Expected outcome after implementation:

- the first response records `send_message` with `sendPath: "direct"`
- the second response records `send_draft` with `sendPath: "draft"`
- both outcomes remain distinguishable from draft-only results

7. Reply to one existing message and forward one message.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "resultMode": "draft",
    "body": "Thanks, following up shortly."
  }' \
  http://127.0.0.1:19192/v1/mail/messages/$MESSAGE_ID/reply
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "resultMode": "send",
    "to": ["carol@example.com"],
    "body": "Forwarding for visibility."
  }' \
  http://127.0.0.1:19192/v1/mail/messages/$MESSAGE_ID/forward
```

Expected outcome after implementation:

- reply preserves linkage to the original thread and returns a draft-only outcome
- forward preserves linkage to the forwarded source and returns a sent outcome
- both operations remain distinct from generic send history

8. Confirm the system rejects unsafe final-send cases truthfully.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "integrationId": "mail-fake-primary",
    "subject": "Missing recipients",
    "body": "This should not send."
  }' \
  http://127.0.0.1:19192/v1/mail/messages/send
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "integrationId": "mail-fake-primary",
    "to": ["bob@example.com"],
    "subject": "Broken attachment reference",
    "body": "This should stay blocked.",
    "attachmentRefs": [
      { "attachmentRefId": "missing-brief", "displayName": "brief.pdf" }
    ]
  }' \
  http://127.0.0.1:19192/v1/mail/messages/send
```

Expected outcome after implementation:

- the first request fails or remains draft-only because explicit recipients are missing
- the second request blocks final send because the attachment reference is unresolved
- both failures stay operator-visible in mail operation history

9. Configure a test delivery target and preference if one does not already exist.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "targetId": "mail-test-sink",
    "displayName": "Mail Test Sink",
    "targetKind": "test_sink",
    "addressSummary": "local://mail-test-sink"
  }' \
  http://127.0.0.1:19192/v1/delivery/targets
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "preferenceId": "mail-default-pref",
    "scopeKind": "user_default",
    "preferredTargetsByClass": {
      "routine_success": "mail-test-sink",
      "urgent": "mail-test-sink",
      "failure": "mail-test-sink"
    }
  }' \
  http://127.0.0.1:19192/v1/delivery/preferences
```

10. Run one background workflow without send permission and one with send permission.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "trigger": {
      "kind": "once",
      "fireAt": "$FIRE_AT_RFC3339"
    },
    "target": {
      "kind": "workflow",
      "workflow": {
        "entrypoint": "operator",
        "runGoal": "mail background draft run",
        "workflowGoal": "Prepare a mail draft without sending.",
        "mailAction": {
          "operationClass": "send_message",
          "integrationId": "mail-fake-primary",
          "to": ["bob@example.com"],
          "subject": "Blocked background send",
          "body": "This should not finalize send.",
          "allowSendSideEffects": false
        }
      }
    },
    "retryPolicy": {
      "maxRetries": 0,
      "backoffKind": "fixed",
      "baseDelaySeconds": 5,
      "maxDelaySeconds": 5
    }
  }' \
  http://127.0.0.1:19192/v1/schedules
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "trigger": {
      "kind": "once",
      "fireAt": "$FIRE_AT_RFC3339"
    },
    "target": {
      "kind": "workflow",
      "workflow": {
        "entrypoint": "operator",
        "runGoal": "mail background send run",
        "workflowGoal": "Send a background mail update.",
        "mailAction": {
          "operationClass": "send_message",
          "integrationId": "mail-fake-primary",
          "to": ["bob@example.com"],
          "subject": "Allowed background send",
          "body": "This should send through the shared delivery plane.",
          "allowSendSideEffects": true
        }
      }
    },
    "retryPolicy": {
      "maxRetries": 0,
      "backoffKind": "fixed",
      "baseDelaySeconds": 5,
      "maxDelaySeconds": 5
    }
  }' \
  http://127.0.0.1:19192/v1/schedules
```

Expected outcome after implementation:

- the first workflow records blocked or draft-only mail truth and does not finalize send
- the second workflow records a sent mail operation and linked delivery outcome
- schedule, workflow, and delivery resources project additive `mailOperationSummaries`
  and `mailOperationIds`

## Observed Results

Observed on `2026-04-23` in `KURA_ENV=test` against a local daemon at `127.0.0.1:19192`.

- Integration `mail-fake-primary` was registered and updated to `healthy` readiness with
  `authorized` auth state.
- `GET /v1/mail/accounts/mail-fake-primary` returned mailbox projection
  `mail_acct_mail-fake-primary` with `mailboxAddress: "alice@example.com"` and all
  capability flags set `true`.
- `GET /v1/mail/threads?integrationId=mail-fake-primary` returned seeded
  `thread_seed`; `GET /v1/mail/messages/msg_seed` returned inbound `msg_seed`; and
  `GET /v1/mail/drafts` returned seeded `draft_seed` without any send side effect.
- Draft create returned `create_draft` with `resultMode: "draft_only"` and stable draft
  id `draft_mail_fake_primary_1776926942679114000`; update preserved the same `draftId`
  and changed `draftStatus` to `updated`.
- Direct send returned `send_message` with `sendPath: "direct"` and message
  `msg_mail_fake_primary_1776926952897073000`.
- Sending the existing draft returned `send_draft` with `sendPath: "draft"`, preserved
  the original draft id, and changed the draft artifact to `sent_from_draft`.
- Reply returned `reply_message` with `resultMode: "draft_only"` and a new reply draft
  linked to `msg_seed`. Forward returned `forward_message` with `resultMode: "sent"`
  and `forwardedFromMessageId: "msg_seed"`.
- New outbound without explicit recipients failed with
  `{"error":"explicit recipients are required for new outbound mail"}`.
- New outbound with unresolved attachment ref `missing-brief` failed with
  `{"error":"attachment reference could not be resolved"}` and persisted blocked mail
  operation truth.
- A foreground workflow without `allowSendSideEffects` failed with
  `lastFailureClass: "send_permission_required"` and a blocked mail operation summary.
- A one-time schedule `sched_25af30e84d8027a9` with
  `allowSendSideEffects: true` completed successfully. Its attempt
  `sched_attempt_c270f0cdbf294ef2` projected mail operation
  `mail_op_ce08803d564b`, `latestDeliveryId: "delivery_eb0e7bad8cd297e6"`, and
  `latestDeliveryStatus: "delivered"`.
- `GET /v1/deliveries?scheduleId=sched_25af30e84d8027a9` returned delivery
  `delivery_eb0e7bad8cd297e6` with `chosenTargetId: "mail-test-sink"`,
  `mailOperationIds: ["mail_op_ce08803d564b"]`, and matching
  `mailOperationSummaries`.

## Notes

- Manual delivery linkage is observable on schedule-driven background workflows. A
  foreground workflow started under an operator run with a non-empty `sessionId`
  intentionally does not emit a delivery outcome; this matches the current
  `maybeEmitWorkflowDelivery(...)` guard in the daemon.
