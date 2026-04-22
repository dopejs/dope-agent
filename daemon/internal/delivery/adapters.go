package delivery

import "context"

type Adapter interface {
	Supports(kind TargetKind) bool
	Send(context.Context, DeliveryTarget, DeliveryOutcome) (SendResult, error)
}
