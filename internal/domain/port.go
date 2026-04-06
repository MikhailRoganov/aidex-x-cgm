package domain

import (
	"context"
	"time"
)

// CGMFetcher is the outbound port for pulling glucose records from the CGM provider.
type CGMFetcher interface {
	// FetchSince returns all CGM records recorded after the given time.
	// pageSize controls how many records to fetch per API page (max 500).
	FetchSince(ctx context.Context, since time.Time) ([]CGMRecord, error)
}

// NightscoutPort is the outbound port for reading from and writing to Nightscout.
type NightscoutPort interface {
	// LastEntryTime returns the timestamp of the most recent SGV entry in Nightscout.
	// Returns zero time and no error if Nightscout has no entries yet.
	LastEntryTime(ctx context.Context) (time.Time, error)

	// PushEntries uploads a slice of SGV entries to Nightscout (idempotent by dateString).
	PushEntries(ctx context.Context, entries []NightscoutEntry) error
}
