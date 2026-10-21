package api

import "github.com/akki/pidesigner/internal/model"

// completeRun closes the process before advertising a terminal state. A new
// prompt must never see "Finished" while the old project lock is still held.
func (r *Runner) completeRun(run model.Run, j *eventJournal, status, message string) {
	j.publish(AgentEvent{Type: "finishing", RunID: run.ID, At: nowMs()})
	r.mu.Lock()
	sess := r.sessions[run.ID]
	r.mu.Unlock()
	if sess != nil {
		sess.Close()
	}
	r.mu.Lock()
	if r.shuttingDown || r.cancelRequested[run.ID] {
		status = model.RunCanceled
		message = ""
	}
	delete(r.sessions, run.ID)
	if r.active[run.ProjectID] == run.ID {
		delete(r.active, run.ProjectID)
	}
	r.mu.Unlock()
	if err := r.store.FinishRun(run.ID, status, message); err != nil {
		j.publish(AgentEvent{Type: "run_failed", RunID: run.ID, Text: "Run state could not be saved: " + err.Error(), At: nowMs()})
		return
	}
	kind := "done"
	if status == model.RunFailed {
		kind = "run_failed"
	}
	if status == model.RunCanceled {
		kind = "canceled"
	}
	j.publish(AgentEvent{Type: kind, RunID: run.ID, Text: message, At: nowMs()})
}
func finishRun(r *Runner, run model.Run, j *eventJournal, aborted bool) {
	status := model.RunSucceeded
	if aborted {
		status = model.RunCanceled
	}
	r.completeRun(run, j, status, "")
}
