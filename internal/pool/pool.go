// Package pool is the round-robin backend pool — the "ports" half of this
// POC. A Pool hands out backends one after another in rotation; how the
// HTTP layer uses it is the balancer's business.
package pool

import (
	"errors"
	"sync/atomic"
)

// Backend is one addressable server behind the balancer.
type Backend struct {
	ID  string // human name, e.g. "backend-1"
	URL string // e.g. "http://127.0.0.1:9001"
}

// Pool rotates through backends evenly. Goroutine-safe.
type Pool struct {
	backends []Backend
	next     atomic.Uint64
}

// New builds a pool; at least one backend is required.
func New(backends []Backend) (*Pool, error) {
	if len(backends) == 0 {
		return nil, errors.New("pool needs at least one backend")
	}
	return &Pool{backends: backends}, nil
}

// Next returns the next backend in rotation.
func (p *Pool) Next() Backend {
	i := p.next.Add(1) - 1
	return p.backends[int(i%uint64(len(p.backends)))]
}

// All returns the full backend list (used by the shard router).
func (p *Pool) All() []Backend { return p.backends }
