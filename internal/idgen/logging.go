package idgen

import (
	"context"
	"log/slog"
)

type loggingAllocator struct {
	next Allocator
	log  *slog.Logger
}

// WithLogging wraps a so every range claim, and every failure, is logged. It keeps logging out of the Generator,
// whose constructor stays free of a logger.
func WithLogging(a Allocator, log *slog.Logger) Allocator {
	return &loggingAllocator{next: a, log: log}
}

// NextRange claims a range from the wrapped allocator and logs the outcome.
func (l *loggingAllocator) NextRange(ctx context.Context, size uint64) (uint64, error) {
	start, err := l.next.NextRange(ctx, size)
	if err != nil {
		l.log.Error("id range refill failed", "size", size, "error", err)
		return 0, err
	}
	l.log.Info("id range refill", "first_id", start+1, "last_id", start+size, "size", size)
	return start, nil
}
