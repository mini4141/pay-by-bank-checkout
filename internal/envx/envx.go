// Package envx reads configuration from environment variables.
package envx

import "os"

func Get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
