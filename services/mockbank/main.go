// Command mockbank simulates an account-to-account bank API: consent,
// reserve, debit, credit. /debit fails randomly according to a fail rate
// (MOCKBANK_FAIL_RATE, adjustable at /admin/fail-rate) so retries are visible.
package main

import (
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/mini4141/pay-by-bank-checkout/internal/envx"
	"github.com/mini4141/pay-by-bank-checkout/internal/logx"
)

// maxConsecutiveFailures keeps the knob recoverable: a payment with a 50%
// fail rate must still settle within the workflow's 5 debit attempts.
const maxConsecutiveFailures = 3

type bank struct {
	mu        sync.Mutex
	failRate  float64
	results   map[string]map[string]any // idempotency key -> response
	failCount map[string]int            // idempotency key -> consecutive injected failures
}

func main() {
	logx.Init("mockbank")
	rate, _ := strconv.ParseFloat(envx.Get("MOCKBANK_FAIL_RATE", "0"), 64)
	b := &bank{failRate: rate, results: map[string]map[string]any{}, failCount: map[string]int{}}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("POST /consent", b.op("consent", false))
	mux.HandleFunc("POST /reserve", b.op("reserve", false))
	mux.HandleFunc("POST /debit", b.op("debit", true))
	mux.HandleFunc("POST /credit", b.op("credit", false))
	mux.HandleFunc("POST /release", b.op("release", false))
	mux.HandleFunc("/admin/fail-rate", b.adminFailRate)

	addr := envx.Get("MOCKBANK_ADDR", ":8081")
	slog.Info("mockbank listening", "addr", addr, "fail_rate", rate)
	srv := &http.Server{Addr: addr, Handler: logx.Middleware(mux), ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func write(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// op handles one bank operation. Every call carries an idempotency_key and
// the first result is replayed on retries, so the worker can retry safely.
func (b *bank) op(name string, injectFailures bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			write(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		key, _ := in["idempotency_key"].(string)
		if key == "" {
			write(w, http.StatusBadRequest, map[string]string{"error": "idempotency_key required"})
			return
		}
		cacheKey := name + ":" + key

		b.mu.Lock()
		defer b.mu.Unlock()
		if res, ok := b.results[cacheKey]; ok {
			slog.Info("replayed", "op", name, "key", key)
			write(w, http.StatusOK, res)
			return
		}
		if injectFailures && b.failRate > 0 && b.failCount[cacheKey] < maxConsecutiveFailures && rand.Float64() < b.failRate {
			b.failCount[cacheKey]++
			slog.Warn("injected_failure", "op", name, "key", key, "consecutive", b.failCount[cacheKey])
			write(w, http.StatusServiceUnavailable, map[string]string{"error": "bank_unavailable"})
			return
		}
		res := map[string]any{"id": name + "_" + key, "status": "OK"}
		if bankID, ok := in["bank"].(string); ok {
			res["bank"] = bankID
		}
		b.results[cacheKey] = res
		slog.Info("ok", "op", name, "key", key, "amount_minor", in["amount_minor"])
		write(w, http.StatusOK, res)
	}
}

func (b *bank) adminFailRate(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if r.Method == http.MethodPost {
		var in struct {
			Rate float64 `json:"rate"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Rate < 0 || in.Rate > 1 {
			write(w, http.StatusBadRequest, map[string]string{"error": "rate must be 0..1"})
			return
		}
		b.failRate = in.Rate
		slog.Info("fail rate changed", "rate", in.Rate)
	}
	write(w, http.StatusOK, map[string]float64{"rate": b.failRate})
}
