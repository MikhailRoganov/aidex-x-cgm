package aidex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/brkss/aidex-x-cgm/config"
	"github.com/brkss/aidex-x-cgm/internal/domain"
)

const (
	maxPageSize    = 500
	readingInterval = time.Minute // AiDEX records one reading per minute
)

// apiResponse mirrors the JSON envelope returned by the AiDEX API.
type apiResponse struct {
	Code  int         `json:"code"`
	Msg   string      `json:"msg"`
	Count int         `json:"count"`
	Data  []apiRecord `json:"data"`
}

type apiRecord struct {
	CGMRecordID    string  `json:"cgmRecordId"`
	UserID         string  `json:"userId"`
	SensorID       string  `json:"sensorId"`
	AppTime        string  `json:"appTime"`        // "2006-01-02 15:04:05" local time
	AppTimeZone    string  `json:"appTimeZone"`    // e.g. "Africa/Casablanca"
	DSTOffset      string  `json:"dstOffset"`      // seconds as string, e.g. "3600"
	AppCreateTime  string  `json:"appCreateTime"`  // "2006-01-02 15:04:05-0700"
	Glucose        float64 `json:"glucose"`
	GlucoseIsValid int     `json:"glucoseIsValid"`
	TrendValue     int     `json:"trendValue"`
	Status         int     `json:"status"`
	Quality        int     `json:"quality"`
}

// parseTime converts an apiRecord timestamp to UTC time.Time.
// It prefers appCreateTime (which carries an explicit offset) and falls back
// to appTime + dstOffset.
func (r apiRecord) parseTime() (time.Time, error) {
	// Try appCreateTime first – it has an explicit TZ offset.
	if r.AppCreateTime != "" {
		t, err := time.Parse("2006-01-02 15:04:05-0700", r.AppCreateTime)
		if err == nil {
			return t.UTC(), nil
		}
	}

	// Fall back: appTime + location derived from appTimeZone.
	layout := "2006-01-02 15:04:05"
	loc := time.UTC
	if r.AppTimeZone != "" {
		if l, err := time.LoadLocation(r.AppTimeZone); err == nil {
			loc = l
		}
	}
	t, err := time.ParseInLocation(layout, r.AppTime, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing appTime %q: %w", r.AppTime, err)
	}
	return t.UTC(), nil
}

// Client is the AiDEX HTTP adapter implementing domain.CGMFetcher.
type Client struct {
	cfg    config.AiDEXConfig
	http   *http.Client
}

func NewClient(cfg config.AiDEXConfig) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchSince returns all records newer than `since` by fetching as many pages
// as needed (AiDEX returns DESC order, newest first).
func (c *Client) FetchSince(ctx context.Context, since time.Time) ([]domain.CGMRecord, error) {
	gap := time.Since(since)
	if gap < 0 {
		gap = readingInterval
	}

	// How many 1-minute readings fit in the gap, plus a small buffer.
	neededRecords := int(gap/readingInterval) + 5
	if neededRecords < 1 {
		neededRecords = 1
	}

	var all []domain.CGMRecord
	pageNum := 1

	for neededRecords > 0 {
		pageSize := neededRecords
		if pageSize > maxPageSize {
			pageSize = maxPageSize
		}

		records, err := c.fetchPage(ctx, pageNum, pageSize)
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			break
		}

		all = append(all, records...)
		neededRecords -= len(records)

		// If the API returned fewer items than requested, we've hit the end.
		if len(records) < pageSize {
			break
		}

		// Stop early if the oldest record on this page is already before `since`.
		oldest := records[len(records)-1].AppTime
		if !oldest.After(since) {
			break
		}

		//pageNum++
	}

	return all, nil
}

func (c *Client) fetchPage(ctx context.Context, pageNum, pageSize int) ([]domain.CGMRecord, error) {
	
	params := url.Values{
		"orderStrategy": {"DESC"},
		"pageNum":       {strconv.Itoa(pageNum)},
		"pageSize":      {strconv.Itoa(pageSize)},
		"userId":        {c.cfg.UserID},
	}

	fullURL := c.cfg.APIURL + "?" + params.Encode()
	log.Printf("[aidex] GET %s", fullURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building aidex request: %w", err)
	}

	// Use direct map assignment to preserve exact lowercase header names.
	// Go's Header.Set() canonicalises names (token→Token, brand→Brand, etc.)
	// but the AiDEX server is case-sensitive on its custom headers.
	req.Header["Accept"] = []string{"*/*"}
	req.Header["Accept-Language"] = []string{"en"}
	req.Header["brand"] = []string{c.cfg.Brand}
	req.Header["token"] = []string{c.cfg.Token}
	req.Header["Content-Type"] = []string{"application/json"}
	req.Header["User-Agent"] = []string{c.cfg.UserAgent}
	req.Header["app-info"] = []string{c.cfg.AppInfo}
	req.Header["encryption"] = []string{"disabled"}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aidex request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading aidex response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[aidex] HTTP %d body: %s", resp.StatusCode, body)
		return nil, fmt.Errorf("aidex returned HTTP %d", resp.StatusCode)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		log.Printf("[aidex] non-JSON body: %s", body)
		return nil, fmt.Errorf("decoding aidex response: %w", err)
	}
	if apiResp.Code != 200 {
		log.Printf("[aidex] API error body: %s, %v, %v\n", body, pageNum, pageSize)
		return nil, fmt.Errorf("aidex API error %d: %s", apiResp.Code, apiResp.Msg)
	}

	records := make([]domain.CGMRecord, 0, len(apiResp.Data))
	for _, r := range apiResp.Data {
		t, err := r.parseTime()
		if err != nil {
			// Skip unparseable records rather than aborting the whole batch.
			continue
		}
		records = append(records, domain.CGMRecord{
			RecordID:     r.CGMRecordID,
			UserID:       r.UserID,
			SensorID:     r.SensorID,
			AppTime:      t,
			Glucose:      r.Glucose,
			GlucoseValid: r.GlucoseIsValid == 1,
			TrendValue:   r.TrendValue,
			Status:       r.Status,
			Quality:      r.Quality,
		})
	}

	return records, nil
}
