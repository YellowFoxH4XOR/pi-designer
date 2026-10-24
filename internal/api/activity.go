package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/akki/pidesigner/internal/model"
	"github.com/akki/pidesigner/internal/pirun"
)

func (s *Server) activityFor(run model.Run) (model.Activity, error) {
	p, err := s.Store.GetProject(run.ProjectID)
	if err != nil {
		return model.Activity{}, err
	}
	a, err := s.Store.RunActivity(run.ID)
	if err != nil {
		a = model.NewActivity(run, p)
		a.Todos = run.Todos
		a.Renders = run.Renders
		a.Critiques = run.Critiques
	}
	a.ProjectName = p.Name
	// Covers legacy runs and a daemon terminated before its terminal journal write.
	if run.Status != "running" && a.Status == "running" || a.Seq == 0 && run.Status != "running" {
		kind := "done"
		if run.Status == "failed" {
			kind = "run_failed"
		}
		if run.Status == "canceled" {
			kind = "canceled"
		}
		at := a.UpdatedAt
		if run.EndedAt != nil {
			at = run.EndedAt.UnixMilli()
		}
		a.Apply(model.AgentEvent{Type: kind, At: at, Seq: a.Seq, Text: run.Error})
	}
	return a, nil
}
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	upper, err := s.Store.OutcomeCursor()
	if err != nil {
		writeErr(w, 500, "Could not load activity")
		return
	}
	outcomes := []model.Activity{}
	cursor := upper
	if raw := r.URL.Query().Get("after"); raw != "" {
		after, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || after < 0 {
			writeErr(w, 400, "Invalid activity cursor")
			return
		}
		events, err := s.Store.Outcomes(after, upper)
		if err != nil {
			writeErr(w, 500, "Could not load activity outcomes")
			return
		}
		if len(events) == 500 {
			cursor = events[len(events)-1].Seq
		}
		for _, event := range events {
			run, err := s.Store.GetRun(event.RunID)
			if err != nil {
				continue
			}
			a, err := s.activityFor(run)
			if err != nil {
				continue
			}
			if event.Type == "attention" && !a.Attention {
				continue
			}
			outcomes = append(outcomes, activitySummary(a))
		}
	}
	runs, err := s.Store.LatestRuns()
	if err != nil {
		writeErr(w, 500, "Could not load activity")
		return
	}
	out := []model.Activity{}
	for _, run := range runs {
		a, err := s.activityFor(run)
		if err != nil {
			continue
		}
		out = append(out, activitySummary(a))
	}
	writeJSON(w, 200, map[string]any{"activities": out, "outcomes": outcomes, "cursor": cursor})
}
func activitySummary(a model.Activity) model.Activity {
	a.Text = ""
	a.TextBlocks = nil
	a.MessageText = ""
	a.Prompt = ""
	a.Tools = nil
	a.Request = nil
	a.Todos = nil
	a.Renders = nil
	a.Critiques = nil
	return a
}

func (s *Server) handleRunActivity(w http.ResponseWriter, r *http.Request) {
	run, err := s.Store.GetRun(r.PathValue("id"))
	if err != nil {
		writeErr(w, 404, "Run not found")
		return
	}
	a, err := s.activityFor(run)
	if err != nil {
		writeErr(w, 500, "Activity could not be loaded")
		return
	}
	writeJSON(w, 200, map[string]any{"activity": a})
}

func (s *Server) handleRunLog(w http.ResponseWriter, r *http.Request) {
	if _, err := s.Store.GetRun(r.PathValue("id")); err != nil {
		writeErr(w, 404, "Run not found")
		return
	}
	after, err := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if r.URL.Query().Get("after") == "" {
		err = nil
	}
	if err != nil || after < 0 {
		writeErr(w, 400, "Invalid event cursor")
		return
	}
	events, err := s.Store.RunEvents(r.PathValue("id"), after, 500)
	if err != nil {
		writeErr(w, 500, "Event log could not be loaded")
		return
	}
	writeJSON(w, 200, map[string]any{"events": events})
}

// Poll the durable journal, not a best-effort channel. Slow clients page from
// their cursor; Last-Event-ID reconnects work after completion and daemon restart.
func (s *Server) handleRunEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.Store.GetRun(id); err != nil {
		writeErr(w, 404, "Run not found")
		return
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if last, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64); last > after {
		after = last
	}
	if after < 0 {
		writeErr(w, 400, "Invalid activity cursor")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	heartbeat := time.Now()
	for {
		events, err := s.Store.RunEvents(id, after, 500)
		if err != nil {
			return
		}
		for _, ev := range events {
			payload, _ := json.Marshal(ev)
			if _, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.Seq, ev.Type, payload); err != nil {
				return
			}
			after = ev.Seq
		}
		flusher.Flush()
		if len(events) == 500 {
			continue
		}
		run, err := s.Store.GetRun(id)
		if err != nil {
			return
		}
		if run.Status != "running" {
			// Terminal state is saved before the final event. Send a final authoritative
			// snapshot so even a crash in that tiny gap cannot strand the subscriber.
			a, err := s.activityFor(run)
			if err == nil {
				payload, _ := json.Marshal(a)
				fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", payload)
				fmt.Fprint(w, "event: stream_end\ndata: {}\n\n")
				flusher.Flush()
			}
			return
		}
		if time.Since(heartbeat) > 10*time.Second {
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
			heartbeat = time.Now()
		}
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
		}
	}
}

func (r *Runner) handleExtensionUI(runID string, sess *pirun.Session, b *eventJournal, data json.RawMessage) {
	var req struct {
		ID      string `json:"id"`
		Method  string `json:"method"`
		Timeout int    `json:"timeout"`
	}
	if json.Unmarshal(data, &req) != nil {
		return
	}
	switch req.Method {
	case "select", "confirm", "input", "editor":
		b.publish(AgentEvent{Type: "attention", RunID: runID, Data: data, At: nowMs()})
		if req.Timeout > 0 {
			time.AfterFunc(time.Duration(req.Timeout)*time.Millisecond, func() {
				a, err := r.store.RunActivity(runID)
				var current struct {
					ID string `json:"id"`
				}
				_ = json.Unmarshal(a.Request, &current)
				if err == nil && current.ID == req.ID {
					b.publish(AgentEvent{Type: "attention_resolved", RunID: runID, At: nowMs()})
				}
			})
		}
	default:
		b.publish(AgentEvent{Type: "extension_ui", RunID: runID, Data: data, At: nowMs()})
	}
}

func (s *Server) handleUIResponse(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var response struct {
		ID        string `json:"id"`
		Value     string `json:"value"`
		Confirmed bool   `json:"confirmed"`
		Cancelled bool   `json:"cancelled"`
	}
	if err := decodeBody(r, &response); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	s.Runner.uiMu.Lock()
	defer s.Runner.uiMu.Unlock()
	a, err := s.Store.RunActivity(r.PathValue("id"))
	if err != nil || !a.Attention || a.Status != "running" {
		writeErr(w, 409, "This request is no longer waiting")
		return
	}
	var req struct {
		ID      string   `json:"id"`
		Method  string   `json:"method"`
		Options []string `json:"options"`
	}
	_ = json.Unmarshal(a.Request, &req)
	if req.ID != response.ID {
		writeErr(w, 409, "This request has already changed")
		return
	}
	if req.Method == "select" && !response.Cancelled {
		valid := false
		for _, o := range req.Options {
			if o == response.Value {
				valid = true
			}
		}
		if !valid {
			writeErr(w, 400, "Choose an available option")
			return
		}
	}
	s.Runner.mu.Lock()
	sess := s.Runner.sessions[a.RunID]
	b := s.Runner.journals[a.RunID]
	s.Runner.mu.Unlock()
	if sess == nil || b == nil {
		writeErr(w, 409, "This run is no longer active")
		return
	}
	out := map[string]any{"id": response.ID, "value": response.Value, "confirmed": response.Confirmed, "cancelled": response.Cancelled}
	if err = sess.RespondUI(out); err != nil {
		writeErr(w, 409, "pi could not receive your answer")
		return
	}
	b.publish(AgentEvent{Type: "attention_resolved", RunID: a.RunID, At: nowMs()})
	writeJSON(w, 200, map[string]bool{"ok": true})
}

var formBlock = regexp.MustCompile(`(?is)<question-form\b[^>]*>(.*?)</question-form\s*>`)

func publishQuestionAttention(id, text string, b *eventJournal) {
	// Markers inside fenced code are examples, never requests for interaction.
	lines := strings.Split(text, "\n")
	fenced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			lines[i] = ""
		} else if fenced {
			lines[i] = ""
		}
	}
	for _, match := range formBlock.FindAllStringSubmatch(strings.Join(lines, "\n"), -1) {
		body := match[1]
		start, end := strings.Index(body, "{"), strings.LastIndex(body, "}")
		if start < 0 || end <= start {
			continue
		}
		var form struct {
			Questions []struct {
				Label string `json:"label"`
				ID    string `json:"id"`
			} `json:"questions"`
		}
		if json.Unmarshal([]byte(body[start:end+1]), &form) == nil {
			for _, q := range form.Questions {
				if q.Label != "" || q.ID != "" {
					b.publish(AgentEvent{Type: "attention", RunID: id, At: nowMs()})
					return
				}
			}
		}
	}
}
