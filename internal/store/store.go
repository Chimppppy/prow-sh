package store

import (
	"context"

	"github.com/prow-sh/prow/pkg/pcs"
)

// EventStore lists and persists PCS events (prowd-only).
type EventStore interface {
	ListEvents(ctx context.Context) ([]pcs.Event, error)
}
