// Package logs provides a bounded log buffer and an event broadcast bus used
// for streaming job lifecycle events over SSE.
package logs

import "sync"

// Ring is a byte-bounded buffer that keeps the most recent log output. Once the
// configured limit is exceeded, the oldest bytes are dropped.
type Ring struct {
	mu    sync.Mutex
	buf   []byte
	limit int
}

// NewRing creates a Ring that retains at most limit bytes.
func NewRing(limit int) *Ring {
	if limit <= 0 {
		limit = 1024 * 1024
	}
	return &Ring{limit: limit}
}

// Write appends p, dropping the oldest bytes if the limit is exceeded. It never
// returns an error so it satisfies io.Writer.
func (r *Ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, p...)
	if len(r.buf) > r.limit {
		r.buf = r.buf[len(r.buf)-r.limit:]
	}
	return len(p), nil
}

// String returns the retained log contents.
func (r *Ring) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.buf)
}
