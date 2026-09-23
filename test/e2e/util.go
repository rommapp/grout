//go:build e2e

package e2e

import "os"

// envOr reads a setting, falling back when it is unset or empty.
func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
