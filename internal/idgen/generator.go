package idgen

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrSpaceExhausted means every id below Space has been issued, so no further unique code exists.
var ErrSpaceExhausted = errors.New("idgen: code space exhausted")

const (
	// prefetchPercent is how much of the current range must be used before the next one is claimed in the background.
	prefetchPercent = 80
	// prefetchTimeout bounds the background claim, which must not depend on the request that happened to trigger it.
	prefetchTimeout = 10 * time.Second
)

// idRange is a run of unused ids: next, next+1, ... for left more ids. The zero value is empty.
type idRange struct {
	next, left uint64
}

// Generator turns allocated ids into short codes. It claims ranges of ids in bulk so most calls never touch the allocator.
type Generator struct {
	alloc Allocator
	size  uint64 // ids per range; must be above zero

	// mu guards every field below. Callers of NextCode wait on it; the prefetch goroutine takes it only to store its result.
	mu       sync.Mutex
	cur      idRange // ids being handed out now
	spare    idRange // ids claimed ahead of time; empty when none
	fetching bool    // true while the prefetch goroutine runs, so at most one runs at a time
}

// NewGenerator returns a Generator that claims rangeSize ids at a time from a. rangeSize must be above zero.
func NewGenerator(a Allocator, rangeSize uint64) *Generator {
	return &Generator{alloc: a, size: rangeSize}
}

// NextCode returns the next unique 7-character code. A crash loses the unused rest of a range, which is harmless.
func (g *Generator) NextCode(ctx context.Context) (string, error) {
	id, err := g.nextID(ctx)
	if err != nil {
		return "", fmt.Errorf("next code: %w", err)
	}
	return Encode(Scramble(id), CodeLen), nil
}

func (g *Generator) nextID(ctx context.Context) (uint64, error) {
	g.mu.Lock() // Blocks while another caller or the prefetch goroutine holds the lock; both hold it briefly or while claiming a range.
	defer g.mu.Unlock()
	if g.cur.left == 0 {
		if err := g.advance(ctx); err != nil {
			return 0, err
		}
	}
	id := g.cur.next
	if id >= Space {
		return 0, ErrSpaceExhausted
	}
	g.cur.next++
	g.cur.left--
	g.prefetchIfNeeded(ctx)
	return id, nil
}

// advance replaces the used-up range with the prefetched one, or claims a new one and waits for it if the prefetch is not ready.
func (g *Generator) advance(ctx context.Context) error {
	if g.spare.left > 0 {
		g.cur, g.spare = g.spare, idRange{}
		return nil
	}
	r, err := g.claim(ctx)
	if err != nil {
		return err
	}
	g.cur = r
	return nil
}

func (g *Generator) claim(ctx context.Context) (idRange, error) {
	start, err := g.alloc.NextRange(ctx, g.size)
	if err != nil {
		return idRange{}, fmt.Errorf("claim id range: %w", err)
	}
	return idRange{next: start + 1, left: g.size}, nil
}

// prefetchIfNeeded starts the background claim once the current range is 80% used. The caller holds g.mu.
func (g *Generator) prefetchIfNeeded(ctx context.Context) {
	used := g.size - g.cur.left
	if g.fetching || g.spare.left > 0 || used*100 < g.size*prefetchPercent {
		return
	}
	g.fetching = true
	// Goroutine owner: this Generator. Started here, by the NextCode call that crosses 80%. It stops by itself when the claim
	// returns or prefetchTimeout passes. It uses no channels: it never sends or receives, nothing closes, and it hands its result
	// over by writing g.spare under g.mu. WithoutCancel keeps the claim alive if the triggering request is cancelled.
	go g.prefetch(context.WithoutCancel(ctx))
}

func (g *Generator) prefetch(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, prefetchTimeout)
	defer cancel()
	r, err := g.claim(ctx) // Waits on the allocator without holding g.mu, so NextCode keeps serving from the current range.
	g.mu.Lock()
	defer g.mu.Unlock()
	g.fetching = false
	if err == nil {
		g.spare = r
	} // On failure the next NextCode call past 80% starts a new attempt; the allocator logs the error.
}
