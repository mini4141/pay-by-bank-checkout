// Command api serves the Connect RPC services. `api migrate` runs the goose
// migrations and exits (used as a one-off task locally and on ECS).
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"
	"go.temporal.io/sdk/client"

	"github.com/mini4141/pay-by-bank-checkout/gen/go/pay/v1/payv1connect"
	"github.com/mini4141/pay-by-bank-checkout/internal/envx"
	"github.com/mini4141/pay-by-bank-checkout/internal/logx"
	"github.com/mini4141/pay-by-bank-checkout/internal/store"
)

const basePath = "/api"

func main() {
	logx.Init("api")
	dbURL := envx.Get("DATABASE_URL", "postgres://pay:pay@localhost:5433/pay?sslmode=disable")

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := store.Migrate(dbURL); err != nil {
			slog.Error("migrate failed", "err", err)
			os.Exit(1)
		}
		slog.Info("migrations applied")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Connect(ctx, dbURL)
	if err != nil {
		slog.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	tc, err := client.Dial(client.Options{
		HostPort:  envx.Get("TEMPORAL_ADDRESS", "localhost:7233"),
		Namespace: envx.Get("TEMPORAL_NAMESPACE", "default"),
	})
	if err != nil {
		slog.Error("temporal dial failed", "err", err)
		os.Exit(1)
	}
	defer tc.Close()

	srv := &server{
		st:          st,
		tc:          tc,
		checkoutURL: strings.TrimRight(envx.Get("CHECKOUT_BASE_URL", "http://localhost:3000"), "/"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	api := http.NewServeMux()
	api.Handle(payv1connect.NewMerchantServiceHandler(srv))
	api.Handle(payv1connect.NewCheckoutServiceHandler(srv))
	api.Handle(payv1connect.NewPaymentServiceHandler(srv))
	mux.Handle(basePath+"/", http.StripPrefix(basePath, api))

	origins := strings.Split(envx.Get("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000"), ",")
	handler := logx.Middleware(cors.New(cors.Options{
		AllowedOrigins: origins,
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: append(connectcors.AllowedHeaders(), "Authorization"),
		ExposedHeaders: connectcors.ExposedHeaders(),
	}).Handler(mux))

	hs := &http.Server{Addr: envx.Get("API_ADDR", ":8080"), Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = hs.Shutdown(sctx)
	}()
	slog.Info("api listening", "addr", hs.Addr)
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
