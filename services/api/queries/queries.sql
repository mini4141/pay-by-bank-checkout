-- name: CreateMerchant :one
INSERT INTO merchants (id, name, api_key_hash, webhook_url, webhook_secret, idempotency_key)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (idempotency_key) DO NOTHING
RETURNING *;

-- name: GetMerchantByIdempotencyKey :one
SELECT * FROM merchants WHERE idempotency_key = $1;

-- name: GetMerchantByKeyHash :one
SELECT * FROM merchants WHERE api_key_hash = $1;

-- name: GetMerchant :one
SELECT * FROM merchants WHERE id = $1;

-- name: CreateCheckout :one
INSERT INTO checkouts (id, merchant_id, amount_minor, currency, reference, return_url, idempotency_key)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (merchant_id, idempotency_key) DO NOTHING
RETURNING *;

-- name: GetCheckoutByIdempotencyKey :one
SELECT * FROM checkouts WHERE merchant_id = $1 AND idempotency_key = $2;

-- name: GetCheckout :one
SELECT c.*, m.name AS merchant_name,
       COALESCE((SELECT p.id FROM payments p WHERE p.checkout_id = c.id), '')::text AS payment_id
FROM checkouts c JOIN merchants m ON m.id = c.merchant_id
WHERE c.id = $1;

-- name: SetCheckoutStatus :exec
UPDATE checkouts SET status = $2 WHERE id = $1;

-- name: CreatePayment :one
INSERT INTO payments (id, checkout_id, bank, workflow_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (checkout_id) DO NOTHING
RETURNING *;

-- name: GetPaymentByCheckout :one
SELECT * FROM payments WHERE checkout_id = $1;

-- name: GetPayment :one
SELECT * FROM payments WHERE id = $1;

-- name: SetPaymentStatus :exec
UPDATE payments SET status = $2, updated_at = now() WHERE id = $1;

-- name: GetPaymentDetail :one
SELECT p.id, p.checkout_id, p.bank, p.status, p.workflow_id, p.created_at, p.updated_at,
       c.amount_minor, c.currency, c.reference, c.merchant_id
FROM payments p JOIN checkouts c ON c.id = p.checkout_id
WHERE p.id = $1;

-- name: ListPaymentDetailsByMerchant :many
SELECT p.id, p.checkout_id, p.bank, p.status, p.workflow_id, p.created_at, p.updated_at,
       c.amount_minor, c.currency, c.reference, c.merchant_id
FROM payments p JOIN checkouts c ON c.id = p.checkout_id
WHERE c.merchant_id = $1
ORDER BY p.created_at DESC
LIMIT 50;

-- name: MerchantFingerprint :one
SELECT COALESCE(max(p.updated_at), 'epoch'::timestamptz)::timestamptz AS last_update, count(*)::bigint AS n
FROM payments p JOIN checkouts c ON c.id = p.checkout_id
WHERE c.merchant_id = $1;

-- name: AddPaymentEvent :exec
INSERT INTO payment_events (payment_id, step, status, detail) VALUES ($1, $2, $3, $4);

-- name: TouchPayment :exec
UPDATE payments SET updated_at = now() WHERE id = $1;

-- name: ListPaymentEvents :many
SELECT * FROM payment_events WHERE payment_id = $1 ORDER BY id;

-- name: ListPaymentEventsForMerchant :many
SELECT e.* FROM payment_events e
JOIN payments p ON p.id = e.payment_id JOIN checkouts c ON c.id = p.checkout_id
WHERE c.merchant_id = $1 ORDER BY e.id;

-- name: AddLedgerEntry :exec
INSERT INTO ledger_entries (merchant_id, payment_id, amount_minor, kind)
VALUES ($1, $2, $3, $4)
ON CONFLICT (payment_id, kind) DO NOTHING;

-- name: GetBalance :one
SELECT COALESCE(sum(amount_minor), 0)::bigint FROM ledger_entries WHERE merchant_id = $1;

-- name: AddWebhookDelivery :exec
INSERT INTO webhook_deliveries (payment_id, event_type, attempt, status_code, error)
VALUES ($1, $2, $3, $4, $5);

-- name: ListWebhookDeliveries :many
SELECT * FROM webhook_deliveries WHERE payment_id = $1 ORDER BY id;

-- name: ListWebhookDeliveriesForMerchant :many
SELECT w.* FROM webhook_deliveries w
JOIN payments p ON p.id = w.payment_id JOIN checkouts c ON c.id = p.checkout_id
WHERE c.merchant_id = $1 ORDER BY w.id;
