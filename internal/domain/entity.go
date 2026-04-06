package domain

import "time"

// CGMRecord represents a single glucose reading from the AiDEX CGM device.
type CGMRecord struct {
	RecordID      string
	UserID        string
	SensorID      string
	AppTime       time.Time // device-local time, converted to UTC
	Glucose       float64   // mg/dL
	GlucoseValid  bool
	TrendValue    int    // mg/dL per minute rate of change
	Status        int
	Quality       int
}

// Direction maps a trendValue (mg/dL/min) to a Nightscout direction string.
func (r CGMRecord) Direction() string {
	switch {
	case r.TrendValue >= 3:
		return "DoubleUp"
	case r.TrendValue == 2:
		return "SingleUp"
	case r.TrendValue == 1:
		return "FortyFiveUp"
	case r.TrendValue == 0:
		return "Flat"
	case r.TrendValue == -1:
		return "FortyFiveDown"
	case r.TrendValue == -2:
		return "SingleDown"
	default:
		return "DoubleDown"
	}
}

// NightscoutEntry is the Nightscout SGV entry format.
type NightscoutEntry struct {
	Type       string  `json:"type"`
	DateString string  `json:"dateString"`
	Date       int64   `json:"date"` // epoch milliseconds
	SGV        int     `json:"sgv"`
	Direction  string  `json:"direction"`
	Noise      int     `json:"noise"`
	Filtered   int     `json:"filtered"`
	Unfiltered int     `json:"unfiltered"`
	RSSI       int     `json:"rssi"`
	Device     string  `json:"device,omitempty"`
}

// ToNightscoutEntry converts a CGMRecord to a NightscoutEntry.
func (r CGMRecord) ToNightscoutEntry() NightscoutEntry {
	utc := r.AppTime.UTC()
	return NightscoutEntry{
		Type:       "sgv",
		DateString: utc.Format("2006-01-02T15:04:05.000Z"),
		Date:       utc.UnixMilli(),
		SGV:        int(r.Glucose),
		Direction:  r.Direction(),
		Noise:      1,
		Filtered:   int(r.Glucose * 1000),
		Unfiltered: int(r.Glucose * 1000),
		RSSI:       100,
		Device:     "AiDEX-X",
	}
}
