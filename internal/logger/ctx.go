package logger

import (
	"context"
	"log/slog"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

func WithRequestID(ctx context.Context, rid string) context.Context {
	return context.WithValue(ctx, requestIDKey, rid)
}

func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

func LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	if rid := RequestIDFrom(ctx); rid != "" {
		attrs = append([]slog.Attr{slog.String("request_id", rid)}, attrs...)
	}
	slog.LogAttrs(ctx, level, msg, attrs...)
}
