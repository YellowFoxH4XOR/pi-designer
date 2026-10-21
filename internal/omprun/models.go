package omprun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Model deliberately excludes endpoints, headers and credentials from omp's
// full model objects. Only presentation/selection metadata crosses the API.
type Model struct {
	ID            string `json:"id"`
	Provider      string `json:"provider"`
	Name          string `json:"name"`
	Reasoning     bool   `json:"reasoning"`
	ContextWindow int    `json:"contextWindow,omitempty"`
}

func (m Model) Ref() string { return m.Provider + "/" + m.ID }

type State struct {
	Model         *Model `json:"model,omitempty"`
	ThinkingLevel string `json:"thinkingLevel"`
}

type ModelList struct {
	Models         []Model  `json:"models"`
	Current        *Model   `json:"current,omitempty"`
	ThinkingLevels []string `json:"thinkingLevels"`
}

// Command handles request correlation and cleans up canceled requests. It does
// not print or expose the raw response: callers decode only the fields needed.
func (s *Session) Command(ctx context.Context, name string, args map[string]any, out any) error {
	id, ch, err := s.send(name, args)
	if err != nil {
		return err
	}
	defer func() { s.mu.Lock(); delete(s.pending, id); s.mu.Unlock() }()
	select {
	case reply := <-ch:
		if !reply.Success {
			return fmt.Errorf("omp %s: %s", name, reply.Error)
		}
		if out != nil && len(reply.Data) > 0 {
			return json.Unmarshal(reply.Data, out)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("omp %s timed out: %w", name, ctx.Err())
	case <-s.done:
		return fmt.Errorf("omp exited during %s: %s", name, s.StderrTail())
	}
}

func (s *Session) State(ctx context.Context) (State, error) {
	var state State
	err := s.Command(ctx, "get_state", nil, &state)
	return state, err
}
func (s *Session) ThinkingLevels(ctx context.Context) ([]string, error) {
	var result struct {
		Levels []string `json:"levels"`
	}
	if err := s.Command(ctx, "get_available_thinking_levels", nil, &result); err != nil {
		return nil, err
	}
	if len(result.Levels) == 0 {
		return nil, fmt.Errorf("omp returned no supported thinking levels")
	}
	return result.Levels, nil
}

// Catalog asks the installed omp for its available models; it never reads auth
// files, calls a model provider directly, or invokes an inference request.
type Catalog struct {
	bin    string
	mu     sync.Mutex
	cached *ModelList
	loaded time.Time
	levels map[string][]string
}

func NewCatalog(bin string) *Catalog { return &Catalog{bin: bin, levels: map[string][]string{}} }

func (c *Catalog) start(ctx context.Context, ref string) (*Session, func(), error) {
	dir, err := os.MkdirTemp("", "pidesigner-model-query-")
	if err != nil {
		return nil, nil, err
	}
	// A metadata-only session: no skills, rules, LSP or tools. Restricting the
	// tool set also skips MCP discovery. User extensions stay loaded because
	// they can register model providers.
	s, err := Start(ctx, Options{Bin: c.bin, Dir: dir, Model: ref,
		ExtraArgs: []string{"--no-skills", "--no-rules", "--no-lsp", "--no-tools"}})
	if err != nil {
		os.RemoveAll(dir)
		return nil, nil, err
	}
	// Metadata-only sessions have no run UI. Drain startup extension events so
	// faithful RPC backpressure cannot prevent command responses from arriving.
	go func() {
		for range s.Events() {
		}
	}()
	return s, func() { s.Close(); os.RemoveAll(dir) }, nil
}

func (c *Catalog) List(ctx context.Context, refresh bool) (ModelList, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cached != nil && !refresh && time.Since(c.loaded) < time.Minute {
		return *c.cached, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	s, cleanup, err := c.start(ctx, "")
	if err != nil {
		return ModelList{}, err
	}
	defer cleanup()
	var data struct {
		Models []Model `json:"models"`
	}
	if err = s.Command(ctx, "get_available_models", nil, &data); err != nil {
		return ModelList{}, err
	}
	state, err := s.State(ctx)
	if err != nil {
		return ModelList{}, err
	}
	levels, err := s.ThinkingLevels(ctx)
	if err != nil {
		return ModelList{}, err
	}
	seen := map[string]bool{}
	models := make([]Model, 0, len(data.Models))
	for _, m := range data.Models {
		if m.Provider == "" || m.ID == "" || seen[m.Ref()] {
			continue
		}
		seen[m.Ref()] = true
		if m.Name == "" {
			m.Name = m.ID
		}
		models = append(models, m)
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].Provider != models[j].Provider {
			return models[i].Provider < models[j].Provider
		}
		return models[i].Name < models[j].Name
	})
	list := ModelList{Models: models, Current: state.Model, ThinkingLevels: levels}
	c.cached = &list
	c.loaded = time.Now()
	c.levels = map[string][]string{"": levels}
	if state.Model != nil {
		c.levels[state.Model.Ref()] = levels
	}
	return list, nil
}

func (c *Catalog) Resolve(ctx context.Context, ref string) (*Model, error) {
	list, err := c.List(ctx, false)
	if err != nil {
		return nil, err
	}
	if ref == "" {
		return list.Current, nil
	}
	for _, m := range list.Models {
		if m.Ref() == ref {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("Model %q is not available in your installed omp. Refresh the model list and choose a listed model", ref)
}
func (c *Catalog) Thinking(ctx context.Context, ref string) ([]string, error) {
	if _, err := c.Resolve(ctx, ref); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if levels, ok := c.levels[ref]; ok {
		return append([]string(nil), levels...), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	s, cleanup, err := c.start(ctx, ref)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	state, err := s.State(ctx)
	if err != nil {
		return nil, err
	}
	if ref != "" && (state.Model == nil || state.Model.Ref() != ref) {
		return nil, fmt.Errorf("omp did not select the requested model %s", ref)
	}
	levels, err := s.ThinkingLevels(ctx)
	if err != nil {
		return nil, err
	}
	c.levels[ref] = levels
	return append([]string(nil), levels...), nil
}
func ContainsLevel(levels []string, value string) bool {
	for _, level := range levels {
		if level == value {
			return true
		}
	}
	return false
}
func ValidateRef(ref string) error {
	if ref == "" {
		return nil
	}
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(ref, "\r\n\x00") {
		return fmt.Errorf("Choose an exact provider/model from the omp model list")
	}
	return nil
}
