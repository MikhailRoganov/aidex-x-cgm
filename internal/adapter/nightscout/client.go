package nightscout

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/brkss/aidex-x-cgm/config"
	"github.com/brkss/aidex-x-cgm/internal/domain"
)

// Client is the Nightscout HTTP adapter implementing domain.NightscoutPort.
type Client struct {
	cfg       config.NightscoutConfig
	http      *http.Client
	apiSecret string // sha1(APISecret)
}

func NewClient(cfg config.NightscoutConfig) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 30 * time.Second,
			// Follow redirects only when the scheme doesn't change (e.g. trailing-slash
			// normalisations). Refuse http→https upgrades so local HTTP setups work.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > 0 && req.URL.Scheme != via[0].URL.Scheme {
					return http.ErrUseLastResponse
				}
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		apiSecret: sha1Hex(cfg.APISecret),
	}
}

// LastEntryTime returns the timestamp of the most recent SGV entry in Nightscout.
// Returns zero time (and no error) when Nightscout has no entries yet.
func (c *Client) LastEntryTime(ctx context.Context) (time.Time, error) {
	url := c.cfg.URL + "/api/v1/entries/sgv.json?count=1"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return time.Time{}, fmt.Errorf("building nightscout request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return time.Time{}, fmt.Errorf("nightscout GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return time.Time{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("nightscout returned HTTP %d", resp.StatusCode)
	}

	var entries []struct {
		Date int64 `json:"date"` // epoch ms
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return time.Time{}, fmt.Errorf("decoding nightscout entries: %w", err)
	}
	if len(entries) == 0 {
		return time.Time{}, nil
	}

	return time.UnixMilli(entries[0].Date).UTC(), nil
}

// PushEntries uploads entries to Nightscout via POST /api/v1/entries.json.
// Nightscout deduplicates by dateString, so re-pushing is safe.
func (c *Client) PushEntries(ctx context.Context, entries []domain.NightscoutEntry) error {
	if len(entries) == 0 {
		return nil
	}

	body, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshalling entries: %w", err)
	}

	url := c.cfg.URL + "/api/v1/entries.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building nightscout POST request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("nightscout POST failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("nightscout POST returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("api-secret", c.apiSecret)
	req.Header.Set("Accept", "application/json")
}

func sha1Hex(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}
