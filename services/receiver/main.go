// Command receiver is a demo merchant webhook endpoint. It verifies the HMAC
// signature, keeps the deliveries in memory and lists them at /receiver/deliveries.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mini4141/pay-by-bank-checkout/internal/envx"
	"github.com/mini4141/pay-by-bank-checkout/internal/logx"
	"github.com/mini4141/pay-by-bank-checkout/internal/webhook"
)

type delivery struct {
	At        time.Time       `json:"at"`
	Signature string          `json:"signature"`
	Valid     bool            `json:"valid"`
	Body      json.RawMessage `json:"body"`
}

func main() {
	logx.Init("receiver")
	secret := envx.Get("WEBHOOK_SECRET", "whsec_demo_local_only")
	var (
		mu         sync.Mutex
		deliveries []delivery
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("POST /receiver/webhook", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		sig := r.Header.Get(webhook.SignatureHeader)
		valid := verify(secret, sig, body)
		mu.Lock()
		deliveries = append([]delivery{{At: time.Now().UTC(), Signature: sig, Valid: valid, Body: body}}, deliveries...)
		mu.Unlock()
		slog.Info("webhook received", "valid", valid)
		if !valid {
			http.Error(w, "bad signature", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /receiver/deliveries", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		out := deliveries
		if out == nil {
			out = []delivery{}
		}
		_ = json.NewEncoder(w).Encode(out)
	})

	addr := envx.Get("RECEIVER_ADDR", ":8082")
	slog.Info("receiver listening", "addr", addr)
	srv := &http.Server{Addr: addr, Handler: logx.Middleware(mux), ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

// verify checks "t=<unix>,v1=<hex hmac-sha256 of t.body>" with a 5 minute tolerance.
func verify(secret, header string, body []byte) bool {
	var ts, v1 string
	for _, part := range strings.Split(header, ",") {
		k, v, _ := strings.Cut(part, "=")
		switch k {
		case "t":
			ts = v
		case "v1":
			v1 = v
		}
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || time.Since(time.Unix(sec, 0)).Abs() > 5*time.Minute {
		return false
	}
	want := webhook.Sign(secret, ts, body)
	got, err := hex.DecodeString(v1)
	exp, _ := hex.DecodeString(want)
	return err == nil && hmac.Equal(got, exp) && len(got) == sha256.Size
}
