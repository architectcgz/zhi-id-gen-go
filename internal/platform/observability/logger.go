package observability

import (
	"log/slog"
	"os"
)

func NewBootstrapLogger(serviceName string) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})).With("service", serviceName)
}
