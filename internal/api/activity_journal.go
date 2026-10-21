package api

import (
	"encoding/json"
	"sync"

	"github.com/akki/pidesigner/internal/model"
	"github.com/akki/pidesigner/internal/store"
)

// eventJournal serializes writes from RPC, extension responses, and timeout
// callbacks. Network consumers read the durable store using their own cursors.
// There is deliberately no best-effort subscriber buffer or replay tail.
type eventJournal struct {
	mu      sync.Mutex
	closed  bool
	persist func(*AgentEvent) error
	err     error
}

func (j *eventJournal) publish(e AgentEvent) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return
	}
	if err := j.persist(&e); err != nil {
		j.err = err
	}
}
func (j *eventJournal) close()                  { j.mu.Lock(); defer j.mu.Unlock(); j.closed = true }
func (j *eventJournal) persistenceError() error { j.mu.Lock(); defer j.mu.Unlock(); return j.err }
func newRunJournal(st *store.Store, run model.Run, project model.Project) *eventJournal {
	activity := model.NewActivity(run, project)
	j := &eventJournal{persist: func(e *AgentEvent) error { return st.AppendRunEvent(e, &activity) }}
	j.publish(AgentEvent{Type: "starting", RunID: run.ID, At: nowMs()})
	return j
}
func modelJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
