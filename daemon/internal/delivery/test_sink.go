package delivery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

type TestSinkMessage struct {
	TargetID       string      `json:"targetId"`
	DeliveryID     string      `json:"deliveryId"`
	ResultClass    ResultClass `json:"resultClass"`
	PayloadPreview string      `json:"payloadPreview"`
	RecordedAt     time.Time   `json:"recordedAt"`
}

type TestSinkAdapter struct {
	mu       sync.Mutex
	messages []TestSinkMessage
}

func NewTestSinkAdapter() *TestSinkAdapter {
	return &TestSinkAdapter{}
}

func (a *TestSinkAdapter) RunLiveValidationOutcome(outcome livevalidation.FakeOutcome) livevalidation.FakeOutcomeResult {
	return livevalidation.FakeOutcomeResultFor(outcome, livevalidation.SafetyClassNonIdempotentMutation)
}

func (a *TestSinkAdapter) Supports(kind TargetKind) bool {
	return kind == TargetKindTestSink
}

func (a *TestSinkAdapter) Send(_ context.Context, target DeliveryTarget, outcome DeliveryOutcome) (SendResult, error) {
	if target.Status != TargetStatusActive {
		return SendResult{TransportKind: string(TargetKindTestSink)}, fmt.Errorf("target %s is not active", target.TargetID)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = append(a.messages, TestSinkMessage{
		TargetID:       target.TargetID,
		DeliveryID:     outcome.DeliveryID,
		ResultClass:    outcome.ResultClass,
		PayloadPreview: outcome.PayloadPreview,
		RecordedAt:     time.Now().UTC(),
	})
	return SendResult{
		TransportKind:  string(TargetKindTestSink),
		ReceiptSummary: "stored in repo-owned test sink",
	}, nil
}

func (a *TestSinkAdapter) Messages() []TestSinkMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	items := make([]TestSinkMessage, 0, len(a.messages))
	items = append(items, a.messages...)
	return items
}
