package stats

import (
	"encoding/json"
	"os"
	"time"
)

// Store holds cumulative per-user traffic statistics, persisted as JSON.
type Store struct {
	Users     map[string]*UserStats `json:"users"`
	UpdatedAt time.Time             `json:"updated_at"`
}

// UserStats tracks a single user's upload and download traffic.
type UserStats struct {
	BaselineUpload   int64                      `json:"baseline_upload"`
	BaselineDownload int64                      `json:"baseline_download"`
	LastRawUpload    int64                      `json:"last_raw_upload"`
	LastRawDownload  int64                      `json:"last_raw_download"`
	Monthly          map[string]*MonthlyTraffic `json:"monthly"`
	TrackedSince     time.Time                  `json:"tracked_since"`
}

// MonthlyTraffic holds traffic for a single calendar month (keyed as "2006-01").
type MonthlyTraffic struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}

// TotalUpload returns cumulative upload bytes across all Xray sessions.
func (u *UserStats) TotalUpload() int64 {
	return u.BaselineUpload + u.LastRawUpload
}

// TotalDownload returns cumulative download bytes across all Xray sessions.
func (u *UserStats) TotalDownload() int64 {
	return u.BaselineDownload + u.LastRawDownload
}

// CurrentMonthTraffic returns upload and download for the month of the given time.
func (u *UserStats) CurrentMonthTraffic(now time.Time) (upload, download int64) {
	key := now.Format("2006-01")
	m, ok := u.Monthly[key]
	if !ok {
		return 0, 0
	}
	return m.Upload, m.Download
}

// LoadStore reads the store from disk. If the file does not exist, an empty
// store is returned.
func LoadStore(path string) (*Store, error) {
	s := &Store{Users: make(map[string]*UserStats)}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.Users == nil {
		s.Users = make(map[string]*UserStats)
	}
	return s, nil
}

// Save writes the store to disk.
func (s *Store) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

// Update incorporates the latest raw counters from Xray for one user.
// It detects Xray restarts (counter going backwards) and accumulates a
// baseline so that totals survive process restarts.
func (s *Store) Update(userName string, rawUpload, rawDownload int64, now time.Time) {
	u := s.Users[userName]
	if u == nil {
		u = &UserStats{
			TrackedSince: now,
			Monthly:      make(map[string]*MonthlyTraffic),
		}
		s.Users[userName] = u
	}

	var deltaUp, deltaDown int64

	if rawUpload < u.LastRawUpload || rawDownload < u.LastRawDownload {
		// Xray restarted — move last snapshot into the baseline.
		u.BaselineUpload += u.LastRawUpload
		u.BaselineDownload += u.LastRawDownload
		deltaUp = rawUpload
		deltaDown = rawDownload
	} else {
		deltaUp = rawUpload - u.LastRawUpload
		deltaDown = rawDownload - u.LastRawDownload
	}

	u.LastRawUpload = rawUpload
	u.LastRawDownload = rawDownload

	// Add delta to the current month's bucket.
	month := now.Format("2006-01")
	m := u.Monthly[month]
	if m == nil {
		m = &MonthlyTraffic{}
		u.Monthly[month] = m
	}
	m.Upload += deltaUp
	m.Download += deltaDown

	s.UpdatedAt = now
}
