// Package ctxlog provides a per-request structured logger stored in context.Context.
package ctxlog

import (
	"context"
	"log/slog"
)

// contextKey is an unexported type for context keys in this package,
// preventing collisions with keys from other packages.
type contextKey string

const (
	CtxKeyRequestID contextKey = "request_id"
	CtxKeyUserID    contextKey = "user_id"
	CtxKeyLogger    contextKey = "logger"
)

// LoggerFromCtx retrieves the per-request logger stored in ctx.
// If no logger is found (e.g. in background goroutines), fallback is returned.
func LoggerFromCtx(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if l, ok := ctx.Value(CtxKeyLogger).(*slog.Logger); ok {
		return l
	}
	return fallback
}
