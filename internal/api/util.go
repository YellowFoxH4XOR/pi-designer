package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// newID returns a short random identifier with the given prefix.
func newID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b)
}

// unsubscribe detaches an SSE subscriber from a run.
func (r *Runner) unsubscribe(runID string, ch chan AgentEvent) {
	r.mu.Lock()
	b, ok := r.brokers[runID]
	r.mu.Unlock()
	if ok {
		b.unsubscribe(ch)
	}
}
