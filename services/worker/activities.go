package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/mini4141/pay-by-bank-checkout/internal/flow"
	"github.com/mini4141/pay-by-bank-checkout/internal/store"
	"github.com/mini4141/pay-by-bank-checkout/internal/webhook"
	"github.com/mini4141/pay-by-bank-checkout/services/api/db"
)

// Activities does all the I/O. Every activity is idempotent: bank calls carry
// an idempotency key of payment id + step, ledger rows are unique per
// (payment, kind), and status writes are plain upserts.
type Activities struct {
	St        *store.Store
	BankURL   string
	RiskLimit int64
	HTTP      *http.Client
}

func (a *Activities) bank(ctx context.Context, path string, body map[string]any) error {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.BankURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("bank %s: %d %s", path, resp.StatusCode, msg)
	}
	return nil
}

func (a *Activities) detail(ctx context.Context, paymentID string) (db.GetPaymentDetailRow, error) {
	return a.St.Q.GetPaymentDetail(ctx, paymentID)
}

func (a *Activities) ReserveFunds(ctx context.Context, paymentID string) error {
	d, err := a.detail(ctx, paymentID)
	if err != nil {
		return err
	}
	if err := a.bank(ctx, "/consent", map[string]any{"bank": d.Bank, "idempotency_key": paymentID}); err != nil {
		return err
	}
	if err := a.bank(ctx, "/reserve", map[string]any{
		"bank": d.Bank, "amount_minor": d.AmountMinor, "currency": d.Currency, "idempotency_key": paymentID,
	}); err != nil {
		return err
	}
	return a.St.RecordEvent(ctx, paymentID, "RESERVE", "OK", "funds reserved at "+d.Bank)
}

func (a *Activities) RiskCheck(ctx context.Context, paymentID string) (bool, error) {
	d, err := a.detail(ctx, paymentID)
	if err != nil {
		return false, err
	}
	if d.AmountMinor > a.RiskLimit {
		return false, a.St.RecordEvent(ctx, paymentID, "RISK", "FAILED", "amount over limit")
	}
	return true, a.St.RecordEvent(ctx, paymentID, "RISK", "OK", "amount within limit")
}

func (a *Activities) Debit(ctx context.Context, paymentID string) error {
	d, err := a.detail(ctx, paymentID)
	if err != nil {
		return err
	}
	if err := a.bank(ctx, "/debit", map[string]any{
		"bank": d.Bank, "amount_minor": d.AmountMinor, "currency": d.Currency, "idempotency_key": paymentID,
	}); err != nil {
		attempt := activity.GetInfo(ctx).Attempt
		_ = a.St.RecordEvent(ctx, paymentID, "DEBIT", "RETRY", fmt.Sprintf("attempt %d failed: %v", attempt, err))
		return err
	}
	return a.St.RecordEvent(ctx, paymentID, "DEBIT", "OK", "debited from "+d.Bank)
}

func (a *Activities) SettleToMerchant(ctx context.Context, paymentID string) error {
	d, err := a.detail(ctx, paymentID)
	if err != nil {
		return err
	}
	return a.St.InTx(ctx, func(q *db.Queries) error {
		if err := q.AddLedgerEntry(ctx, db.AddLedgerEntryParams{MerchantID: d.MerchantID, PaymentID: paymentID, AmountMinor: d.AmountMinor, Kind: "PAYMENT"}); err != nil {
			return err
		}
		if err := q.SetPaymentStatus(ctx, db.SetPaymentStatusParams{ID: paymentID, Status: flow.StatusSettled}); err != nil {
			return err
		}
		if err := q.SetCheckoutStatus(ctx, db.SetCheckoutStatusParams{ID: d.CheckoutID, Status: "PAID"}); err != nil {
			return err
		}
		return store.RecordEventTx(ctx, q, paymentID, "SETTLE", "OK", "credited merchant ledger")
	})
}

func (a *Activities) ReleaseReservation(ctx context.Context, paymentID string) error {
	if err := a.bank(ctx, "/release", map[string]any{"idempotency_key": paymentID}); err != nil {
		return err
	}
	return a.St.RecordEvent(ctx, paymentID, "RESERVE", "RELEASED", "reservation released")
}

// MarkPayment moves a payment to a terminal failure state.
func (a *Activities) MarkPayment(ctx context.Context, paymentID, status, detail string) error {
	d, err := a.detail(ctx, paymentID)
	if err != nil {
		return err
	}
	return a.St.InTx(ctx, func(q *db.Queries) error {
		if err := q.SetPaymentStatus(ctx, db.SetPaymentStatusParams{ID: paymentID, Status: status}); err != nil {
			return err
		}
		if err := q.SetCheckoutStatus(ctx, db.SetCheckoutStatusParams{ID: d.CheckoutID, Status: status}); err != nil {
			return err
		}
		return store.RecordEventTx(ctx, q, paymentID, "SETTLE", "FAILED", detail)
	})
}

// DeliverWebhook posts an HMAC-signed event to the merchant and records each attempt.
func (a *Activities) DeliverWebhook(ctx context.Context, paymentID, eventType string) error {
	d, err := a.detail(ctx, paymentID)
	if err != nil {
		return err
	}
	m, err := a.St.Q.GetMerchant(ctx, d.MerchantID)
	if err != nil {
		return err
	}
	if m.WebhookUrl == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]any{
		"id": paymentID + ":" + eventType, "type": eventType, "payment_id": paymentID, "checkout_id": d.CheckoutID,
		"reference": d.Reference, "amount_minor": d.AmountMinor, "currency": d.Currency, "status": d.Status,
	})
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.WebhookUrl, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(webhook.SignatureHeader, fmt.Sprintf("t=%s,v1=%s", ts, webhook.Sign(m.WebhookSecret, ts, body)))

	attempt := activity.GetInfo(ctx).Attempt
	code, errMsg := 0, ""
	resp, err := a.HTTP.Do(req)
	if err != nil {
		errMsg = err.Error()
	} else {
		resp.Body.Close()
		code = resp.StatusCode
		if code/100 != 2 {
			errMsg = "non-2xx response"
		}
	}
	if rerr := a.St.Q.AddWebhookDelivery(ctx, db.AddWebhookDeliveryParams{
		PaymentID: paymentID, EventType: eventType, Attempt: attempt, StatusCode: int32(code), Error: errMsg,
	}); rerr != nil {
		return rerr
	}
	_ = a.St.Q.TouchPayment(ctx, paymentID)
	if errMsg != "" {
		return fmt.Errorf("webhook delivery failed: %s (%d)", errMsg, code)
	}
	return nil
}

// ---- refund activities ----

func (a *Activities) BeginRefund(ctx context.Context, paymentID string) error {
	if err := a.St.Q.SetPaymentStatus(ctx, db.SetPaymentStatusParams{ID: paymentID, Status: flow.StatusRefunding}); err != nil {
		return err
	}
	return a.St.RecordEvent(ctx, paymentID, "REFUND", "RETRY", "refund started")
}

func (a *Activities) Credit(ctx context.Context, paymentID string) error {
	d, err := a.detail(ctx, paymentID)
	if err != nil {
		return err
	}
	return a.bank(ctx, "/credit", map[string]any{
		"bank": d.Bank, "amount_minor": d.AmountMinor, "currency": d.Currency, "idempotency_key": paymentID,
	})
}

func (a *Activities) LedgerRefund(ctx context.Context, paymentID string) error {
	d, err := a.detail(ctx, paymentID)
	if err != nil {
		return err
	}
	return a.St.InTx(ctx, func(q *db.Queries) error {
		if err := q.AddLedgerEntry(ctx, db.AddLedgerEntryParams{MerchantID: d.MerchantID, PaymentID: paymentID, AmountMinor: -d.AmountMinor, Kind: "REFUND"}); err != nil {
			return err
		}
		if err := q.SetPaymentStatus(ctx, db.SetPaymentStatusParams{ID: paymentID, Status: flow.StatusRefunded}); err != nil {
			return err
		}
		if err := q.SetCheckoutStatus(ctx, db.SetCheckoutStatusParams{ID: d.CheckoutID, Status: flow.StatusRefunded}); err != nil {
			return err
		}
		return store.RecordEventTx(ctx, q, paymentID, "REFUND", "OK", "shopper credited, ledger reversed")
	})
}
