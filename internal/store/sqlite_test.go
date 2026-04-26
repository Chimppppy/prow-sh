package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSQLiteMigrateSeedAndList(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dsn := "file:" + filepath.ToSlash(filepath.Join(dir, "t.db")) + "?_pragma=busy_timeout(5000)"

	st, err := OpenSQLite(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := SeedSampleEvents(ctx, st); err != nil {
		t.Fatal(err)
	}
	ev, err := st.ListEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev) != 3 {
		t.Fatalf("expected 3 seeded events, got %d", len(ev))
	}
}
