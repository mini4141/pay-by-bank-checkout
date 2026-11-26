// Package flow holds the names and inputs shared by the API (which starts
// workflows) and the worker (which runs them). It deliberately has no
// Temporal workflow code so the API never links activity dependencies.
package flow

const (
	TaskQueue       = "payments"
	PaymentWorkflow = "PaymentWorkflow"
	RefundWorkflow  = "RefundWorkflow"
	SignalCancel    = "cancel"
	QueryStatus     = "status"
)

// Payment status values stored in payments.status.
const (
	StatusPending   = "PENDING"
	StatusSettled   = "SETTLED"
	StatusFailed    = "FAILED"
	StatusCancelled = "CANCELLED"
	StatusRefunding = "REFUNDING"
	StatusRefunded  = "REFUNDED"
)

// PaymentWorkflowID makes duplicate submissions attach to the same workflow.
func PaymentWorkflowID(checkoutID string) string { return "payment-" + checkoutID }

func RefundWorkflowID(paymentID string) string { return "refund-" + paymentID }

type PaymentInput struct {
	PaymentID string
}

type RefundInput struct {
	PaymentID string
}
