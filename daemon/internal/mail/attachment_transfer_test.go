package mail

import (
	"errors"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

// FR-005: attachment policy fails over-limit and unsafe attachments explicitly.
func TestEvaluateAttachmentPolicy(t *testing.T) {
	if r := EvaluateAttachment("report.pdf", "application/pdf", 1024); r.Status != AttachmentResolutionResolved {
		t.Fatalf("in-policy attachment should resolve: %+v", r)
	}
	tooBig := EvaluateAttachment("big.pdf", "application/pdf", MaxAttachmentBytes+1)
	if tooBig.Status != AttachmentResolutionFailed || tooBig.FailureReason == "" {
		t.Fatalf("over-limit should fail: %+v", tooBig)
	}
	unsafe := EvaluateAttachment("payload.exe", "application/x-msdownload", 10)
	if unsafe.Status != AttachmentResolutionFailed {
		t.Fatalf("executable should fail: %+v", unsafe)
	}
}

// US1 (FR-001/FR-002, SC-001/SC-002): download produces a managed artifact under policy;
// over-policy fails explicitly with no partial transfer.
func TestDownloadAttachmentUnderPolicy(t *testing.T) {
	m := NewManager("test")
	resources := []integrations.Resource{healthyMailResource("mail-a", true)}

	_, ref, op, artifacts, err := m.DownloadAttachment(resources, DownloadAttachmentInput{
		Selection: Selection{IntegrationID: "mail-a"}, MessageID: "msg-1",
		AttachmentRefID: "att-1", DisplayName: "report.pdf", MediaType: "application/pdf", SizeBytes: 2048,
	})
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if op.OperationClass != OperationClassDownloadAttachment || op.Status != OperationStatusCompleted {
		t.Fatalf("download op wrong: %+v", op)
	}
	if !ref.Downloaded || ref.RetentionClass != RetentionClassStandard || len(artifacts) != 1 {
		t.Fatalf("managed artifact not produced: ref=%+v artifacts=%d", ref, len(artifacts))
	}

	_, overRef, overOp, _, overErr := m.DownloadAttachment(resources, DownloadAttachmentInput{
		Selection: Selection{IntegrationID: "mail-a"}, MessageID: "msg-1",
		AttachmentRefID: "att-2", DisplayName: "huge.bin", SizeBytes: MaxAttachmentBytes + 1,
	})
	if !errors.Is(overErr, ErrMailAttachmentUnresolved) {
		t.Fatalf("over-limit download should fail: %v", overErr)
	}
	if overOp.Status != OperationStatusFailed || overRef.ResolutionStatus == AttachmentResolutionResolved {
		t.Fatalf("over-limit must not resolve: op=%+v ref=%+v", overOp, overRef)
	}
}

// US2 (FR-004/FR-005, SC-003): a send with an over-policy attachment is blocked (no partial).
func TestSendBlockedByAttachmentPolicy(t *testing.T) {
	m := NewManager("test")
	resources := []integrations.Resource{healthyMailResource("mail-a", true)}

	_, _, op, _, err := m.SendMessage(resources, SendMessageInput{
		Selection: Selection{IntegrationID: "mail-a"},
		To:        []string{"b@example.com"}, Subject: "Hi", Body: "hello",
		AttachmentRefs: []AttachmentRefInput{{AttachmentRefID: "a1", DisplayName: "huge.pdf", MediaType: "application/pdf", SizeBytes: MaxAttachmentBytes + 1}},
	})
	if err == nil {
		t.Fatal("expected blocked send")
	}
	if op.Status != OperationStatusBlocked || op.FailureClass != "attachment_unresolved" {
		t.Fatalf("send not blocked on attachment policy: %+v", op)
	}
}
