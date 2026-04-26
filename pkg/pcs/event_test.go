package pcs

import (
	"encoding/json"
	"testing"
)

func TestParseSeverity(t *testing.T) {
	got, err := ParseSeverity("HIGH")
	if err != nil {
		t.Fatal(err)
	}
	if got != SeverityHigh {
		t.Fatalf("got %q want %q", got, SeverityHigh)
	}
	if _, err := ParseSeverity("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSeverityValid(t *testing.T) {
	if !SeverityMedium.Valid() {
		t.Fatal("medium should be valid")
	}
	if Severity("bogus").Valid() {
		t.Fatal("bogus should be invalid")
	}
}

func TestEventUnmarshalJSONSeverity(t *testing.T) {
	const in = `{"event_id":"evt_x","tenant_id":"tnt_lab","ingested_at":"2026-04-26T12:00:00Z","occurred_at":"2026-04-26T11:00:00Z","source":{"connector":"seed"},"category":"alert","severity":"critical","title":"t","summary":"s"}`
	var e Event
	if err := json.Unmarshal([]byte(in), &e); err != nil {
		t.Fatal(err)
	}
	if e.Severity != SeverityCritical {
		t.Fatalf("severity %q", e.Severity)
	}
	if err := json.Unmarshal([]byte(`{"event_id":"x","tenant_id":"t","ingested_at":"2026-04-26T12:00:00Z","occurred_at":"2026-04-26T11:00:00Z","source":{},"category":"a","severity":"invalid","title":"","summary":""}`), &e); err == nil {
		t.Fatal("expected invalid severity error")
	}
}
