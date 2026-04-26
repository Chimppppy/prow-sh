package store

import (
	"context"
	"crypto/rand"
	sql "database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/spf13/viper"
	_ "modernc.org/sqlite"

	"github.com/prow-sh/prow/pkg/pcs"
)

const defaultTenantID = "tnt_lab"

// SQLiteStore is SQLite-backed event storage for lab mode.
type SQLiteStore struct {
	db *sql.DB
}

// OpenSQLite opens (or creates) a SQLite database file.
func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS events (
			event_id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			ingested_at TEXT NOT NULL,
			occurred_at TEXT NOT NULL,
			source_json TEXT NOT NULL,
			category TEXT NOT NULL,
			severity TEXT NOT NULL,
			title TEXT NOT NULL,
			summary TEXT NOT NULL,
			tags_json TEXT NOT NULL DEFAULT '[]',
			labels_json TEXT NOT NULL DEFAULT '{}'
		);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// ListEvents returns all events ordered by occurred_at descending.
func (s *SQLiteStore) ListEvents(ctx context.Context) ([]pcs.Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT event_id, tenant_id, ingested_at, occurred_at, source_json, category, severity, title, summary, tags_json, labels_json
		FROM events
		ORDER BY occurred_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []pcs.Event
	for rows.Next() {
		var (
			e            pcs.Event
			ing, occ     string
			sourceJSON   string
			sev          string
			tags, labels string
		)
		if err := rows.Scan(&e.EventID, &e.TenantID, &ing, &occ, &sourceJSON, &e.Category, &sev, &e.Title, &e.Summary, &tags, &labels); err != nil {
			return nil, err
		}
		e.Severity = pcs.Severity(sev)
		if !e.Severity.Valid() {
			return nil, fmt.Errorf("invalid severity in db for %s: %q", e.EventID, sev)
		}
		e.IngestedAt, err = time.Parse(time.RFC3339Nano, ing)
		if err != nil {
			e.IngestedAt, _ = time.Parse(time.RFC3339, ing)
		}
		e.OccurredAt, err = time.Parse(time.RFC3339Nano, occ)
		if err != nil {
			e.OccurredAt, _ = time.Parse(time.RFC3339, occ)
		}
		if err := json.Unmarshal([]byte(sourceJSON), &e.Source); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(tags), &e.Tags); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(labels), &e.Labels); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Close releases the database handle.
func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// InsertEvent inserts a single PCS event.
func (s *SQLiteStore) InsertEvent(ctx context.Context, e pcs.Event) error {
	sourceJSON, err := json.Marshal(e.Source)
	if err != nil {
		return err
	}
	tagsJSON, err := json.Marshal(e.Tags)
	if err != nil {
		return err
	}
	labelsJSON, err := json.Marshal(e.Labels)
	if err != nil {
		return err
	}
	if e.Labels == nil {
		labelsJSON = []byte("{}")
	}
	if e.Tags == nil {
		tagsJSON = []byte("[]")
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO events (event_id, tenant_id, ingested_at, occurred_at, source_json, category, severity, title, summary, tags_json, labels_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.EventID, e.TenantID, e.IngestedAt.UTC().Format(time.RFC3339Nano), e.OccurredAt.UTC().Format(time.RFC3339Nano),
		string(sourceJSON), e.Category, string(e.Severity), e.Title, e.Summary, string(tagsJSON), string(labelsJSON),
	)
	return err
}

// SeedSampleEvents inserts demo events used before connectors exist.
func SeedSampleEvents(ctx context.Context, s *SQLiteStore) error {
	now := time.Now().UTC()
	samples := []pcs.Event{
		{
			EventID:    newEventID(),
			TenantID:   defaultTenantID,
			IngestedAt: now.Add(-2 * time.Minute),
			OccurredAt: now.Add(-5 * time.Minute),
			Source:     pcs.Source{Connector: "seed", VendorEventID: "demo-001"},
			Category:   "alert",
			Severity:   pcs.SeverityHigh,
			Title:      "Suspicious login pattern",
			Summary:    "Multiple failed authentications followed by a successful login from a new region.",
			Tags:       []string{"auth", "lab"},
			Labels:     map[string]string{"env": "lab"},
		},
		{
			EventID:    newEventID(),
			TenantID:   defaultTenantID,
			IngestedAt: now.Add(-10 * time.Minute),
			OccurredAt: now.Add(-20 * time.Minute),
			Source:     pcs.Source{Connector: "seed", VendorEventID: "demo-002"},
			Category:   "telemetry",
			Severity:   pcs.SeverityInfo,
			Title:      "Host heartbeat",
			Summary:    "Periodic health check from enrolled endpoint.",
			Tags:       []string{"endpoint"},
			Labels:     map[string]string{"hostname": "lab-host-01"},
		},
		{
			EventID:    newEventID(),
			TenantID:   defaultTenantID,
			IngestedAt: now.Add(-1 * time.Hour),
			OccurredAt: now.Add(-90 * time.Minute),
			Source:     pcs.Source{Connector: "seed", VendorEventID: "demo-003"},
			Category:   "vuln",
			Severity:   pcs.SeverityMedium,
			Title:      "OpenSSL advisory (sample)",
			Summary:    "Sample vulnerability finding for Phase 0A scaffold display.",
			Tags:       []string{"vuln", "openssl"},
			Labels:     map[string]string{"cve": "CVE-0000-00000"},
		},
	}
	for _, e := range samples {
		if err := s.InsertEvent(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

func newEventID() string {
	return "evt_" + ulid.Make().String()
}

// LabPaths are well-known Phase 0A lab file locations under the user's home directory.
type LabPaths struct {
	ProDir       string
	ConfigPath   string
	DBPath       string
	TokenPath    string
	Token        string
	BindAddr     string
	SQLiteDSNOut string
}

// InitLab creates ~/.prow, config, token, SQLite DB, migrations, and seed events.
func InitLab() (*LabPaths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	proDir := filepath.Join(home, ".prow")
	if err := os.MkdirAll(proDir, 0o700); err != nil {
		return nil, err
	}

	token, err := generateLabToken()
	if err != nil {
		return nil, err
	}
	tokenPath := filepath.Join(proDir, "prowd.token")
	if err := os.WriteFile(tokenPath, []byte(strings.TrimSpace(token)+"\n"), 0o600); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(proDir, "prowd.db")
	// modernc.org/sqlite DSN: file path with pragma busy_timeout
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", filepath.ToSlash(dbPath))
	st, err := OpenSQLite(dsn)
	if err != nil {
		return nil, err
	}
	defer st.Close()

	ctx := context.Background()
	var n int
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		if err := SeedSampleEvents(ctx, st); err != nil {
			return nil, err
		}
	}

	bind := "127.0.0.1:7777"
	cfgPath := filepath.Join(proDir, "prowd-config.yaml")
	v := viper.New()
	v.Set("mode", "lab")
	v.Set("server.bind", bind)
	v.Set("storage.sqlite_dsn", dsn)
	v.Set("auth.token_file", tokenPath)
	if err := v.WriteConfigAs(cfgPath); err != nil {
		return nil, err
	}

	return &LabPaths{
		ProDir:       proDir,
		ConfigPath:   cfgPath,
		DBPath:       dbPath,
		TokenPath:    tokenPath,
		Token:        token,
		BindAddr:     bind,
		SQLiteDSNOut: dsn,
	}, nil
}

func generateLabToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
