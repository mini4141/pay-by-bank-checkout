// Package store wraps the pgx pool and the sqlc queries.
package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/mini4141/pay-by-bank-checkout/services/api/db"
	"github.com/mini4141/pay-by-bank-checkout/services/api/migrations"
)

type Store struct {
	Pool *pgxpool.Pool
	Q    *db.Queries
}

func Connect(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &Store{Pool: pool, Q: db.New(pool)}, nil
}

// Migrate applies the embedded goose migrations.
func Migrate(url string) error {
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return err
	}
	conn := stdlib.OpenDB(*cfg)
	defer conn.Close()
	return migrateDB(conn)
}

func migrateDB(conn *sql.DB) error {
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(conn, ".")
}

// RecordEvent appends a payment event and bumps payments.updated_at atomically,
// which is what the streaming RPCs poll on (see ADR 0001).
func (s *Store) RecordEvent(ctx context.Context, paymentID, step, status, detail string) error {
	return s.InTx(ctx, func(q *db.Queries) error {
		return recordEvent(ctx, q, paymentID, step, status, detail)
	})
}

func recordEvent(ctx context.Context, q *db.Queries, paymentID, step, status, detail string) error {
	if err := q.AddPaymentEvent(ctx, db.AddPaymentEventParams{PaymentID: paymentID, Step: step, Status: status, Detail: detail}); err != nil {
		return err
	}
	return q.TouchPayment(ctx, paymentID)
}

// RecordEventTx is RecordEvent inside a caller-provided transaction.
func RecordEventTx(ctx context.Context, q *db.Queries, paymentID, step, status, detail string) error {
	return recordEvent(ctx, q, paymentID, step, status, detail)
}

func (s *Store) InTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := fn(s.Q.WithTx(tx)); err != nil {
		return fmt.Errorf("tx: %w", err)
	}
	return tx.Commit(ctx)
}
