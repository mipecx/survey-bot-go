package ctxlog

import (
	"context"
	"log/slog"
)

type contextKey string

const (
	CtxKeyRequestID contextKey = "request_id"
	CtxKeyUserID    contextKey = "user_id"
	CtxKeyLogger    contextKey = "logger"
)

// LoggerFromCtx taking logger from the context
func LoggerFromCtx(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if l, ok := ctx.Value(CtxKeyLogger).(*slog.Logger); ok {
		return l
	}
	return fallback
}
