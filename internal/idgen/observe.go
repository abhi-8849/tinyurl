package idgen

import "context"

// RefillObserver is told about every range claim, so it can be counted.
type RefillObserver interface {
	RangeRefilled()
	RangeRefillFailed()
}

type observedAllocator struct {
	next Allocator
	obs  RefillObserver
}

// WithObserver wraps a so every range claim, and every failure, is reported to o.
func WithObserver(a Allocator, o RefillObserver) Allocator {
	return &observedAllocator{next: a, obs: o}
}

// NextRange claims a range from the wrapped allocator and reports the outcome.
func (o *observedAllocator) NextRange(ctx context.Context, size uint64) (uint64, error) {
	start, err := o.next.NextRange(ctx, size)
	if err != nil {
		o.obs.RangeRefillFailed()
		return 0, err
	}
	o.obs.RangeRefilled()
	return start, nil
}
