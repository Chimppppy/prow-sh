package pcs

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Severity is the normalized PCS severity ladder.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Valid returns true if s is a known PCS severity.
func (s Severity) Valid() bool {
	switch s {
	case SeverityInfo, SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return true
	default:
		return false
	}
}

// ParseSeverity normalizes and validates a severity string.
func ParseSeverity(raw string) (Severity, error) {
	v := Severity(strings.ToLower(strings.TrimSpace(raw)))
	if !v.Valid() {
		return "", fmt.Errorf("invalid severity %q: want info|low|medium|high|critical", raw)
	}
	return v, nil
}

// Source identifies where an event originated (connector + vendor id).
type Source struct {
	Connector     string `json:"connector"`
	VendorEventID string `json:"vendor_event_id,omitempty"`
}

// Event is a minimal PCS v0.1 event (JSON-friendly).
type Event struct {
	EventID    string            `json:"event_id"`
	TenantID   string            `json:"tenant_id"`
	IngestedAt time.Time         `json:"ingested_at"`
	OccurredAt time.Time         `json:"occurred_at"`
	Source     Source            `json:"source"`
	Category   string            `json:"category"`
	Severity   Severity          `json:"severity"`
	Title      string            `json:"title"`
	Summary    string            `json:"summary"`
	Tags       []string          `json:"tags,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// UnmarshalJSON accepts string severity values.
func (e *Event) UnmarshalJSON(data []byte) error {
	type raw struct {
		EventID    string            `json:"event_id"`
		TenantID   string            `json:"tenant_id"`
		IngestedAt time.Time         `json:"ingested_at"`
		OccurredAt time.Time         `json:"occurred_at"`
		Source     Source            `json:"source"`
		Category   string            `json:"category"`
		Severity   string            `json:"severity"`
		Title      string            `json:"title"`
		Summary    string            `json:"summary"`
		Tags       []string          `json:"tags,omitempty"`
		Labels     map[string]string `json:"labels,omitempty"`
	}
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	sev, err := ParseSeverity(r.Severity)
	if err != nil {
		return err
	}
	e.EventID = r.EventID
	e.TenantID = r.TenantID
	e.IngestedAt = r.IngestedAt
	e.OccurredAt = r.OccurredAt
	e.Source = r.Source
	e.Category = r.Category
	e.Severity = sev
	e.Title = r.Title
	e.Summary = r.Summary
	e.Tags = r.Tags
	e.Labels = r.Labels
	return nil
}
