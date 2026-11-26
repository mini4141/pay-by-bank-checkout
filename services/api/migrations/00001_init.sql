-- +goose Up
CREATE TABLE merchants (
  id              text PRIMARY KEY,
  name            text NOT NULL,
  api_key_hash    text NOT NULL UNIQUE,
  webhook_url     text NOT NULL,
  webhook_secret  text NOT NULL,
  idempotency_key text NOT NULL UNIQUE,
  created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE checkouts (
  id              text PRIMARY KEY,
  merchant_id     text NOT NULL REFERENCES merchants(id),
  amount_minor    bigint NOT NULL CHECK (amount_minor > 0),
  currency        text NOT NULL,
  reference       text NOT NULL,
  return_url      text NOT NULL,
  status          text NOT NULL DEFAULT 'OPEN',
  idempotency_key text NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (merchant_id, idempotency_key)
);

CREATE TABLE payments (
  id          text PRIMARY KEY,
  checkout_id text NOT NULL UNIQUE REFERENCES checkouts(id),
  bank        text NOT NULL,
  status      text NOT NULL DEFAULT 'PENDING',
  workflow_id text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE payment_events (
  id         bigserial PRIMARY KEY,
  payment_id text NOT NULL REFERENCES payments(id),
  step       text NOT NULL,
  status     text NOT NULL,
  detail     text NOT NULL DEFAULT '',
  at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payment_events_payment_idx ON payment_events (payment_id, id);

CREATE TABLE ledger_entries (
  id           bigserial PRIMARY KEY,
  merchant_id  text NOT NULL REFERENCES merchants(id),
  payment_id   text NOT NULL REFERENCES payments(id),
  amount_minor bigint NOT NULL,
  kind         text NOT NULL, -- PAYMENT (+) or REFUND (-)
  at           timestamptz NOT NULL DEFAULT now(),
  UNIQUE (payment_id, kind)
);

CREATE TABLE webhook_deliveries (
  id          bigserial PRIMARY KEY,
  payment_id  text NOT NULL REFERENCES payments(id),
  event_type  text NOT NULL,
  attempt     int NOT NULL,
  status_code int NOT NULL DEFAULT 0,
  error       text NOT NULL DEFAULT '',
  at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX webhook_deliveries_payment_idx ON webhook_deliveries (payment_id, id);

-- +goose Down
DROP TABLE webhook_deliveries, ledger_entries, payment_events, payments, checkouts, merchants;
