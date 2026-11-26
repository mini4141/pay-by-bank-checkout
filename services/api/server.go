package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"google.golang.org/protobuf/types/known/timestamppb"

	payv1 "github.com/mini4141/pay-by-bank-checkout/gen/go/pay/v1"
	"github.com/mini4141/pay-by-bank-checkout/internal/flow"
	"github.com/mini4141/pay-by-bank-checkout/internal/store"
	"github.com/mini4141/pay-by-bank-checkout/services/api/db"
)

// server implements MerchantService, CheckoutService and PaymentService.
type server struct {
	st          *store.Store
	tc          client.Client
	checkoutURL string
}

var banks = map[string]bool{"sakura": true, "fuji": true, "aoba": true}

const (
	pollEvery    = 250 * time.Millisecond
	streamMaxAge = 20 * time.Second // under ALB/CloudFront idle timeouts; clients reconnect
)

func newID(prefix string) string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func hashKey(k string) string {
	h := sha256.Sum256([]byte(k))
	return hex.EncodeToString(h[:])
}

func (s *server) auth(ctx context.Context, h http.Header) (db.Merchant, error) {
	key := strings.TrimPrefix(h.Get("Authorization"), "Bearer ")
	if key == "" {
		return db.Merchant{}, connect.NewError(connect.CodeUnauthenticated, errors.New("missing API key"))
	}
	m, err := s.st.Q.GetMerchantByKeyHash(ctx, hashKey(key))
	if err != nil {
		return db.Merchant{}, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid API key"))
	}
	return m, nil
}

func internal(err error) error { return connect.NewError(connect.CodeInternal, err) }

// ---- MerchantService ----

func (s *server) CreateMerchant(ctx context.Context, req *connect.Request[payv1.CreateMerchantRequest]) (*connect.Response[payv1.CreateMerchantResponse], error) {
	r := req.Msg
	if r.Name == "" || r.IdempotencyKey == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name and idempotency_key are required"))
	}
	secret := r.WebhookSecret
	if secret == "" {
		secret = newID("whsec_")
	}
	apiKey := newID("sk_live_")
	m, err := s.st.Q.CreateMerchant(ctx, db.CreateMerchantParams{
		ID: newID("mer_"), Name: r.Name, ApiKeyHash: hashKey(apiKey),
		WebhookUrl: r.WebhookUrl, WebhookSecret: secret, IdempotencyKey: r.IdempotencyKey,
	})
	if errors.Is(err, pgx.ErrNoRows) { // idempotent replay: the key is never returned twice
		m, err = s.st.Q.GetMerchantByIdempotencyKey(ctx, r.IdempotencyKey)
		apiKey = ""
	}
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&payv1.CreateMerchantResponse{MerchantId: m.ID, ApiKey: apiKey, WebhookSecret: m.WebhookSecret}), nil
}

func (s *server) GetBalance(ctx context.Context, req *connect.Request[payv1.GetBalanceRequest]) (*connect.Response[payv1.GetBalanceResponse], error) {
	m, err := s.auth(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	bal, err := s.st.Q.GetBalance(ctx, m.ID)
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&payv1.GetBalanceResponse{
		Balance: &payv1.Money{AmountMinor: bal, Currency: "JPY"}, MerchantName: m.Name,
	}), nil
}

// ---- CheckoutService ----

func (s *server) toCheckout(c db.GetCheckoutRow) *payv1.Checkout {
	return &payv1.Checkout{
		Id: c.ID, MerchantName: c.MerchantName, Amount: &payv1.Money{AmountMinor: c.AmountMinor, Currency: c.Currency},
		Reference: c.Reference, ReturnUrl: c.ReturnUrl, Status: c.Status,
		CheckoutUrl: s.checkoutURL + "/pay/" + c.ID, PaymentId: c.PaymentID,
	}
}

func (s *server) CreateCheckout(ctx context.Context, req *connect.Request[payv1.CreateCheckoutRequest]) (*connect.Response[payv1.CreateCheckoutResponse], error) {
	m, err := s.auth(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	r := req.Msg
	if r.Amount == nil || r.Amount.AmountMinor <= 0 || len(r.Amount.Currency) != 3 || r.IdempotencyKey == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("amount (minor units + ISO currency) and idempotency_key are required"))
	}
	c, err := s.st.Q.CreateCheckout(ctx, db.CreateCheckoutParams{
		ID: newID("chk_"), MerchantID: m.ID, AmountMinor: r.Amount.AmountMinor, Currency: strings.ToUpper(r.Amount.Currency),
		Reference: r.Reference, ReturnUrl: r.ReturnUrl, IdempotencyKey: r.IdempotencyKey,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		c, err = s.st.Q.GetCheckoutByIdempotencyKey(ctx, db.GetCheckoutByIdempotencyKeyParams{MerchantID: m.ID, IdempotencyKey: r.IdempotencyKey})
	}
	if err != nil {
		return nil, internal(err)
	}
	row, err := s.st.Q.GetCheckout(ctx, c.ID)
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&payv1.CreateCheckoutResponse{Checkout: s.toCheckout(row)}), nil
}

func (s *server) GetCheckout(ctx context.Context, req *connect.Request[payv1.GetCheckoutRequest]) (*connect.Response[payv1.GetCheckoutResponse], error) {
	row, err := s.st.Q.GetCheckout(ctx, req.Msg.CheckoutId)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("checkout not found"))
	}
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&payv1.GetCheckoutResponse{Checkout: s.toCheckout(row)}), nil
}

// ---- PaymentService ----

func (s *server) StartPayment(ctx context.Context, req *connect.Request[payv1.StartPaymentRequest]) (*connect.Response[payv1.StartPaymentResponse], error) {
	r := req.Msg
	if !banks[r.Bank] {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unknown bank"))
	}
	co, err := s.st.Q.GetCheckout(ctx, r.CheckoutId)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("checkout not found"))
	}
	if err != nil {
		return nil, internal(err)
	}
	wfID := flow.PaymentWorkflowID(co.ID)
	// A duplicate submission hits the checkout_id unique constraint and
	// re-attaches to the existing payment and workflow (ADR 0002).
	p, err := s.st.Q.CreatePayment(ctx, db.CreatePaymentParams{ID: newID("pay_"), CheckoutID: co.ID, Bank: r.Bank, WorkflowID: wfID})
	if errors.Is(err, pgx.ErrNoRows) {
		p, err = s.st.Q.GetPaymentByCheckout(ctx, co.ID)
	}
	if err != nil {
		return nil, internal(err)
	}
	_, err = s.tc.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       wfID,
		TaskQueue:                flow.TaskQueue,
		WorkflowIDConflictPolicy: enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
		WorkflowIDReusePolicy:    enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}, flow.PaymentWorkflow, flow.PaymentInput{PaymentID: p.ID})
	var already *serviceerror.WorkflowExecutionAlreadyStarted
	if err != nil && !errors.As(err, &already) {
		return nil, internal(err)
	}
	pay, err := s.loadPayment(ctx, p.ID)
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&payv1.StartPaymentResponse{Payment: pay}), nil
}

func (s *server) WatchPayment(ctx context.Context, req *connect.Request[payv1.WatchPaymentRequest], stream *connect.ServerStream[payv1.WatchPaymentResponse]) error {
	var last string
	return s.poll(ctx, func() (bool, error) {
		p, err := s.loadPayment(ctx, req.Msg.PaymentId)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, connect.NewError(connect.CodeNotFound, errors.New("payment not found"))
		}
		if err != nil {
			return false, internal(err)
		}
		if fp := fingerprint(p); fp != last {
			last = fp
			if err := stream.Send(&payv1.WatchPaymentResponse{Payment: p}); err != nil {
				return false, err
			}
		}
		return p.Status != flow.StatusPending, nil
	})
}

func (s *server) ListPayments(ctx context.Context, req *connect.Request[payv1.ListPaymentsRequest]) (*connect.Response[payv1.ListPaymentsResponse], error) {
	m, err := s.auth(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	ps, err := s.loadMerchantPayments(ctx, m.ID)
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&payv1.ListPaymentsResponse{Payments: ps}), nil
}

func (s *server) WatchMerchantPayments(ctx context.Context, req *connect.Request[payv1.WatchMerchantPaymentsRequest], stream *connect.ServerStream[payv1.WatchMerchantPaymentsResponse]) error {
	m, err := s.auth(ctx, req.Header())
	if err != nil {
		return err
	}
	var last string
	return s.poll(ctx, func() (bool, error) {
		fp, err := s.st.Q.MerchantFingerprint(ctx, m.ID)
		if err != nil {
			return false, internal(err)
		}
		cur := fmt.Sprintf("%d/%d", fp.LastUpdate.Time.UnixNano(), fp.N)
		if cur == last {
			return false, nil
		}
		last = cur
		ps, err := s.loadMerchantPayments(ctx, m.ID)
		if err != nil {
			return false, internal(err)
		}
		return false, stream.Send(&payv1.WatchMerchantPaymentsResponse{Payments: ps})
	})
}

func (s *server) RefundPayment(ctx context.Context, req *connect.Request[payv1.RefundPaymentRequest]) (*connect.Response[payv1.RefundPaymentResponse], error) {
	m, err := s.auth(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	d, err := s.st.Q.GetPaymentDetail(ctx, req.Msg.PaymentId)
	if err != nil || d.MerchantID != m.ID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("payment not found"))
	}
	if d.Status != flow.StatusSettled && d.Status != flow.StatusRefunding && d.Status != flow.StatusRefunded {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("only settled payments can be refunded"))
	}
	_, err = s.tc.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       flow.RefundWorkflowID(d.ID),
		TaskQueue:                flow.TaskQueue,
		WorkflowIDConflictPolicy: enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
		WorkflowIDReusePolicy:    enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}, flow.RefundWorkflow, flow.RefundInput{PaymentID: d.ID})
	var already *serviceerror.WorkflowExecutionAlreadyStarted
	if err != nil && !errors.As(err, &already) {
		return nil, internal(err)
	}
	p, err := s.loadPayment(ctx, d.ID)
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&payv1.RefundPaymentResponse{Payment: p}), nil
}

// ---- helpers ----

// poll runs fn every pollEvery until it reports done, errors, the client goes
// away, or streamMaxAge elapses (the client then re-subscribes).
func (s *server) poll(ctx context.Context, fn func() (done bool, err error)) error {
	ctx, cancel := context.WithTimeout(ctx, streamMaxAge)
	defer cancel()
	t := time.NewTicker(pollEvery)
	defer t.Stop()
	for {
		done, err := fn()
		if err != nil || done {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func fingerprint(p *payv1.Payment) string {
	return fmt.Sprintf("%s/%d/%d/%d", p.Status, p.UpdatedAt.AsTime().UnixNano(), len(p.Events), len(p.Deliveries))
}

func toPayment(d db.GetPaymentDetailRow, evs []db.PaymentEvent, dels []db.WebhookDelivery) *payv1.Payment {
	p := &payv1.Payment{
		Id: d.ID, CheckoutId: d.CheckoutID, Bank: d.Bank, Status: d.Status, WorkflowId: d.WorkflowID,
		Amount:    &payv1.Money{AmountMinor: d.AmountMinor, Currency: d.Currency},
		Reference: d.Reference, CreatedAt: timestamppb.New(d.CreatedAt.Time), UpdatedAt: timestamppb.New(d.UpdatedAt.Time),
	}
	for _, e := range evs {
		p.Events = append(p.Events, &payv1.PaymentEvent{Id: e.ID, Step: e.Step, Status: e.Status, Detail: e.Detail, At: timestamppb.New(e.At.Time)})
	}
	for _, w := range dels {
		p.Deliveries = append(p.Deliveries, &payv1.WebhookDelivery{Id: w.ID, EventType: w.EventType, Attempt: w.Attempt, StatusCode: w.StatusCode, Error: w.Error, At: timestamppb.New(w.At.Time)})
	}
	return p
}

func (s *server) loadPayment(ctx context.Context, id string) (*payv1.Payment, error) {
	d, err := s.st.Q.GetPaymentDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	evs, err := s.st.Q.ListPaymentEvents(ctx, id)
	if err != nil {
		return nil, err
	}
	dels, err := s.st.Q.ListWebhookDeliveries(ctx, id)
	if err != nil {
		return nil, err
	}
	return toPayment(d, evs, dels), nil
}

func (s *server) loadMerchantPayments(ctx context.Context, merchantID string) ([]*payv1.Payment, error) {
	rows, err := s.st.Q.ListPaymentDetailsByMerchant(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	evs, err := s.st.Q.ListPaymentEventsForMerchant(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	dels, err := s.st.Q.ListWebhookDeliveriesForMerchant(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	evBy := map[string][]db.PaymentEvent{}
	for _, e := range evs {
		evBy[e.PaymentID] = append(evBy[e.PaymentID], e)
	}
	delBy := map[string][]db.WebhookDelivery{}
	for _, w := range dels {
		delBy[w.PaymentID] = append(delBy[w.PaymentID], w)
	}
	out := make([]*payv1.Payment, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPayment(db.GetPaymentDetailRow(r), evBy[r.ID], delBy[r.ID]))
	}
	return out, nil
}
