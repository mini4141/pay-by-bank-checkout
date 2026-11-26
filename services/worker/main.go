// Command worker runs the Temporal workflows and activities.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"go.temporal.io/sdk/worker"

	"github.com/mini4141/pay-by-bank-checkout/internal/envx"
	"github.com/mini4141/pay-by-bank-checkout/internal/flow"
	"github.com/mini4141/pay-by-bank-checkout/internal/logx"
	"github.com/mini4141/pay-by-bank-checkout/internal/store"
	"github.com/mini4141/pay-by-bank-checkout/internal/temporalx"
)

func main() {
	logx.Init("worker")
	ctx := context.Background()

	st, err := store.Connect(ctx, envx.Get("DATABASE_URL", "postgres://pay:pay@localhost:5433/pay?sslmode=disable"))
	if err != nil {
		slog.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	limit, _ := strconv.ParseInt(envx.Get("RISK_LIMIT_MINOR", "1000000"), 10, 64)
	acts := &Activities{
		St:        st,
		BankURL:   envx.Get("MOCKBANK_URL", "http://localhost:8081"),
		RiskLimit: limit,
		HTTP:      &http.Client{Timeout: 8 * time.Second},
	}

	c, err := temporalx.Dial()
	if err != nil {
		slog.Error("temporal dial failed", "err", err)
		os.Exit(1)
	}
	defer c.Close()

	w := worker.New(c, flow.TaskQueue, worker.Options{})
	w.RegisterWorkflow(PaymentWorkflow) // registered under its function name == flow.PaymentWorkflow
	w.RegisterWorkflow(RefundWorkflow)
	w.RegisterActivity(acts)

	// Health endpoint for container orchestrators.
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
		_ = http.ListenAndServe(envx.Get("WORKER_HEALTH_ADDR", ":8083"), mux)
	}()

	slog.Info("worker started", "task_queue", flow.TaskQueue)
	if err := w.Run(worker.InterruptCh()); err != nil {
		slog.Error("worker stopped", "err", err)
		os.Exit(1)
	}
}
