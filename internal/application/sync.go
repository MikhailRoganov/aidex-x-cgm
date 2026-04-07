package application

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/brkss/aidex-x-cgm/internal/domain"
)

const (
	// maxLookback caps initial sync to avoid fetching years of data.
	maxLookback = 24 * time.Hour
	// minGap avoids redundant pushes when Nightscout is up-to-date.
	minGap = 30 * time.Second
)

// SyncService orchestrates the pull-from-CGM → push-to-Nightscout workflow.
type SyncService struct {
	cgm        domain.CGMFetcher
	nightscout domain.NightscoutPort
}

func NewSyncService(cgm domain.CGMFetcher, ns domain.NightscoutPort) *SyncService {
	return &SyncService{cgm: cgm, nightscout: ns}
}

// Sync performs one synchronisation cycle.
func (s *SyncService) Sync(ctx context.Context) error {
	// 1. Determine how far back we need to fetch.
	lastEntry, err := s.nightscout.LastEntryTime(ctx)
	if err != nil {
		return fmt.Errorf("querying nightscout last entry: %w", err)
	}

	now := time.Now().UTC()
	var since time.Time

	if lastEntry.IsZero() {
		// First run: pull up to maxLookback of history.
		since = now.Add(-maxLookback)
		log.Printf("[sync] nightscout is empty – fetching last %v of data", maxLookback)
	} else {
		gap := now.Sub(lastEntry)
		if gap < minGap {
			log.Printf("[sync] nightscout is up-to-date (gap=%v) – skipping", gap.Round(time.Second))
			return nil
		}
		since = lastEntry
		log.Printf("[sync] last nightscout entry was %v ago – fetching since %v", gap.Round(time.Second), since.Format(time.RFC3339))
	}

	// 2. Pull CGM records covering the gap.
	records, err := s.cgm.FetchSince(ctx, since)
	if err != nil {
		return fmt.Errorf("fetching cgm records: %w", err)
	}

	if len(records) == 0 {
		log.Println("[sync] no new CGM records found")
		return nil
	}

	// 2.1 Sort records chronologically (ASC). 
	// AiDEX returns newest first (DESC), but we process them oldest first.
	sort.Slice(records, func(i, j int) bool {
		return records[i].AppTime.Before(records[j].AppTime)
	})

	// 3. Convert and filter to only records strictly after the last Nightscout entry.
	var entries []domain.NightscoutEntry
	for _, r := range records {
		if !r.GlucoseValid {
			continue
		}
		if !lastEntry.IsZero() && !r.AppTime.After(lastEntry) {
			continue
		}
		entries = append(entries, r.ToNightscoutEntry())
	}

	if len(entries) == 0 {
		log.Println("[sync] no new entries after deduplication")
		return nil
	}

	// 4. Push to Nightscout.
	log.Printf("[sync] pushing %d entries to nightscout", len(entries))
	if err := s.nightscout.PushEntries(ctx, entries); err != nil {
		return fmt.Errorf("pushing entries to nightscout: %w", err)
	}

	log.Printf("[sync] done – pushed %d entries", len(entries))
	return nil
}
