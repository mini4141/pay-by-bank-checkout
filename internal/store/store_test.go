package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mini4141/pay-by-bank-checkout/internal/store"
	"github.com/mini4141/pay-by-bank-checkout/services/api/db"
)

// Integration test: needs Postgres (compose, or the CI service container).
func TestInsertAndReadPayment(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	require.NoError(t, store.Migrate(url))
	st, err := store.Connect(ctx, url)
	require.NoError(t, err)
	defer st.Pool.Close()

	suffix := t.Name() + t.TempDir() // unique per run
	m, err := st.Q.CreateMerchant(ctx, db.CreateMerchantParams{ID: "mer_it" + suffix[len(suffix)-8:], Name: "IT", ApiKeyHash: "h" + suffix, WebhookUrl: "", WebhookSecret: "s", IdempotencyKey: "k" + suffix})
	require.NoError(t, err)
	c, err := st.Q.CreateCheckout(ctx, db.CreateCheckoutParams{ID: "chk_" + m.ID, MerchantID: m.ID, AmountMinor: 1200, Currency: "JPY", Reference: "r", ReturnUrl: "", IdempotencyKey: "i"})
	require.NoError(t, err)
	p, err := st.Q.CreatePayment(ctx, db.CreatePaymentParams{ID: "pay_" + m.ID, CheckoutID: c.ID, Bank: "sakura", WorkflowID: "payment-" + c.ID})
	require.NoError(t, err)
	require.NoError(t, st.RecordEvent(ctx, p.ID, "RESERVE", "OK", "x"))

	got, err := st.Q.GetPaymentDetail(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1200), got.AmountMinor)
	require.Equal(t, "PENDING", got.Status)
	evs, err := st.Q.ListPaymentEvents(ctx, p.ID)
	require.NoError(t, err)
	require.Len(t, evs, 1)
}
