// Package temporalx dials the Temporal server, waiting while it starts up.
package temporalx

import (
	"log/slog"
	"time"

	"go.temporal.io/sdk/client"

	"github.com/mini4141/pay-by-bank-checkout/internal/envx"
)

// Dial retries for about a minute; on ECS the server and its clients start together.
func Dial() (client.Client, error) {
	var err error
	for i := 0; i < 30; i++ {
		var c client.Client
		c, err = client.Dial(client.Options{
			HostPort:  envx.Get("TEMPORAL_ADDRESS", "localhost:7233"),
			Namespace: envx.Get("TEMPORAL_NAMESPACE", "default"),
		})
		if err == nil {
			return c, nil
		}
		slog.Warn("temporal not ready", "err", err)
		time.Sleep(2 * time.Second)
	}
	return nil, err
}
