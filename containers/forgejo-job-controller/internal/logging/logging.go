// Package logging is the structured JSON logger shared by the controller. Every line carries the service name and a
// trace ID, so a poll cycle can be followed across the calls it makes to Forgejo and the Kubernetes API.
package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
)

// Service is the service name written on every log line and sent as the caller name on outbound calls.
const Service = "forgejo-job-controller"

type traceKey struct{}

var base = slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", Service)

// Init sets the minimum level of the logger.
func Init(level slog.Level) {
	base = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})).With("service", Service)
}

// NewTraceID returns a random 128-bit trace ID as 32 hex characters (the W3C trace-id format).
func NewTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// WithTrace returns a context that carries the trace ID.
func WithTrace(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceKey{}, id)
}

// TraceID returns the trace ID of the context, or an empty string when there is none.
func TraceID(ctx context.Context) string {
	id, _ := ctx.Value(traceKey{}).(string)
	return id
}

// From returns a logger that adds the trace ID of the context to every line.
func From(ctx context.Context) *slog.Logger {
	return base.With("trace_id", TraceID(ctx))
}
