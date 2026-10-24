// Package model holds the domain types shared across the daemon.
package model

import "time"

// Project is a design workspace: a directory on disk plus its metadata.
type Project struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	Dir            string    `json:"-"`
	DesignSystemID string    `json:"designSystemId,omitempty"`
	SkillID        string    `json:"skillId,omitempty"`
	EntryFile      string    `json:"entryFile,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Conversation is a thread of messages inside a project.
type Conversation struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"projectId"`
	Title      string    `json:"title,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	MessageCnt int       `json:"messageCount"`
}

// Message roles.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleEvent     = "event"
)

// Message is one turn in a conversation. Delta messages stream partial text.
type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"createdAt"`
}

// Run statuses.
const (
	RunRunning   = "running"
	RunSucceeded = "succeeded"
	RunFailed    = "failed"
	RunCanceled  = "canceled"
)

// Run tracks one agent invocation against a project.
type Run struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"projectId"`
	ConversationID string     `json:"conversationId,omitempty"`
	Status         string     `json:"status"`
	Prompt         string     `json:"prompt,omitempty"`
	Error          string     `json:"error,omitempty"`
	StartedAt      time.Time  `json:"startedAt"`
	EndedAt        *time.Time `json:"endedAt,omitempty"`
	// Todos is the agent's latest plan snapshot.
	Todos []Todo `json:"todos,omitempty"`
	// Renders are the previews the agent rendered to inspect its work.
	Renders []Render `json:"renders,omitempty"`
	// Critiques are the expert panel critique rounds, in order.
	Critiques []Critique `json:"critiques,omitempty"`
}

// Todo is one step of the agent's plan.
type Todo struct {
	Content string `json:"content"`
	Status  string `json:"status"` // pending | in_progress | completed
}

// Render is one preview the agent rendered and inspected.
type Render struct {
	Viewport  string `json:"viewport"`
	Path      string `json:"path"`
	Image     string `json:"image"` // daemon URL of the PNG
	FullPage  bool   `json:"fullPage,omitempty"`
	Slide     int    `json:"slide,omitempty"` // deck slide shown, 1-based
	At        int    `json:"at,omitempty"`    // motion time captured, in ms
	OverflowX bool   `json:"overflowX"`
	Errors    int    `json:"errors"`
}

// Critique is one round of the expert panel critique.
type Critique struct {
	Round int `json:"round"`
	// Composite is the score that decided the round: the independent jury's
	// when one ran, otherwise the agent's own panel.
	Composite       float64     `json:"composite"`
	SelfComposite   float64     `json:"selfComposite,omitempty"`
	Decision        string      `json:"decision"` // revise | ship | ship_with_issues
	SignatureMoment string      `json:"signatureMoment,omitempty"`
	Panelists       []Panelist  `json:"panelists"`
	Jury            *JuryReview `json:"jury,omitempty"`
}

// JuryReview is an independent model's Awwwards-rubric score of fresh renders.
type JuryReview struct {
	Model   string             `json:"model"`
	Scores  map[string]float64 `json:"scores"`
	Overall float64            `json:"overall"`
	Verdict string             `json:"verdict"`
	Issues  []string           `json:"issues"`
}

// Panelist is one reviewer's verdict in a critique round.
type Panelist struct {
	Role    string   `json:"role"` // designer | critic | brand | a11y | copy
	Score   *float64 `json:"score,omitempty"`
	Notes   string   `json:"notes"`
	MustFix []string `json:"mustFix"`
}

// File is a project file entry as reported to clients.
type File struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	Kind string `json:"kind"`
	Mime string `json:"mime,omitempty"`
}

// DesignSystem is a brand package from the content catalog.
type DesignSystem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    string   `json:"category,omitempty"`
	Description string   `json:"description,omitempty"`
	Slug        string   `json:"slug"`
	Source      string   `json:"source,omitempty"`
	HasPreview  bool     `json:"hasPreview"`
	Files       []string `json:"files,omitempty"`
}

// Skill is an agent-behavior pack from the content catalog.
type Skill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Triggers    []string `json:"triggers,omitempty"`
	Mode        string   `json:"mode,omitempty"`
	Scenario    string   `json:"scenario,omitempty"`
	Path        string   `json:"-"`
	HasScripts  bool     `json:"hasScripts"`
}
