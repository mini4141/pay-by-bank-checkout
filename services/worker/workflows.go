package main

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/mini4141/pay-by-bank-checkout/internal/flow"
)

const (
	eventSucceeded = "payment.succeeded"
	eventFailed    = "payment.failed"
	eventRefunded  = "payment.refunded"
)

func quick(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 500 * time.Millisecond, MaximumAttempts: 3},
	})
}

// debitOpts: initial 1s, max 5 attempts (SPECS F3).
func debitOpts(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second, BackoffCoefficient: 2, MaximumInterval: 8 * time.Second, MaximumAttempts: 5,
		},
	})
}

// webhookOpts: retry up to 10 times.
func webhookOpts(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second, BackoffCoefficient: 2, MaximumInterval: 15 * time.Second, MaximumAttempts: 10,
		},
	})
}

// PaymentWorkflow: reserve -> risk -> debit -> settle -> webhook.
// It is deterministic: all I/O happens in activities keyed by payment id.
func PaymentWorkflow(ctx workflow.Context, in flow.PaymentInput) error {
	step := "STARTED"
	if err := workflow.SetQueryHandler(ctx, flow.QueryStatus, func() (string, error) { return step, nil }); err != nil {
		return err
	}
	cancelCh := workflow.GetSignalChannel(ctx, flow.SignalCancel)
	var a *Activities
	id := in.PaymentID

	fail := func(status, detail string) error {
		step = status
		if err := workflow.ExecuteActivity(quick(ctx), a.MarkPayment, id, status, detail).Get(ctx, nil); err != nil {
			return err
		}
		// Best effort: the payment is already terminal, so a webhook failure is not fatal.
		_ = workflow.ExecuteActivity(webhookOpts(ctx), a.DeliverWebhook, id, eventFailed).Get(ctx, nil)
		return nil
	}

	step = "RESERVE"
	if err := workflow.ExecuteActivity(quick(ctx), a.ReserveFunds, id).Get(ctx, nil); err != nil {
		return fail(flow.StatusFailed, "reserve failed")
	}

	step = "RISK"
	var approved bool
	if err := workflow.ExecuteActivity(quick(ctx), a.RiskCheck, id).Get(ctx, &approved); err != nil || !approved {
		// Compensation: give the reserved funds back.
		if err := workflow.ExecuteActivity(quick(ctx), a.ReleaseReservation, id).Get(ctx, nil); err != nil {
			return err
		}
		return fail(flow.StatusFailed, "risk check rejected")
	}

	if cancelCh.ReceiveAsync(nil) { // cancel is only honoured before the debit
		if err := workflow.ExecuteActivity(quick(ctx), a.ReleaseReservation, id).Get(ctx, nil); err != nil {
			return err
		}
		return fail(flow.StatusCancelled, "cancelled by signal")
	}

	step = "DEBIT"
	if err := workflow.ExecuteActivity(debitOpts(ctx), a.Debit, id).Get(ctx, nil); err != nil {
		if rerr := workflow.ExecuteActivity(quick(ctx), a.ReleaseReservation, id).Get(ctx, nil); rerr != nil {
			return rerr
		}
		return fail(flow.StatusFailed, "debit failed")
	}

	step = "SETTLE"
	if err := workflow.ExecuteActivity(quick(ctx), a.SettleToMerchant, id).Get(ctx, nil); err != nil {
		return err
	}

	step = "WEBHOOK"
	if err := workflow.ExecuteActivity(webhookOpts(ctx), a.DeliverWebhook, id, eventSucceeded).Get(ctx, nil); err != nil {
		step = "SETTLED_WEBHOOK_FAILED" // money is settled; the merchant can still read the dashboard
		return nil
	}
	step = flow.StatusSettled
	return nil
}

// RefundWorkflow credits the shopper's bank, reverses the ledger and notifies the merchant.
func RefundWorkflow(ctx workflow.Context, in flow.RefundInput) error {
	step := "STARTED"
	if err := workflow.SetQueryHandler(ctx, flow.QueryStatus, func() (string, error) { return step, nil }); err != nil {
		return err
	}
	var a *Activities
	id := in.PaymentID

	if err := workflow.ExecuteActivity(quick(ctx), a.BeginRefund, id).Get(ctx, nil); err != nil {
		return err
	}
	step = "CREDIT"
	if err := workflow.ExecuteActivity(debitOpts(ctx), a.Credit, id).Get(ctx, nil); err != nil {
		return err
	}
	step = "LEDGER"
	if err := workflow.ExecuteActivity(quick(ctx), a.LedgerRefund, id).Get(ctx, nil); err != nil {
		return err
	}
	step = "WEBHOOK"
	_ = workflow.ExecuteActivity(webhookOpts(ctx), a.DeliverWebhook, id, eventRefunded).Get(ctx, nil)
	step = flow.StatusRefunded
	return nil
}
