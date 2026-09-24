package picoshare

import (
	"fmt"
	"time"
)

const MaxDownloadHistoryRetentionDays = 3650

var ErrInvalidDownloadHistoryRetention = fmt.Errorf("download history retention must be between 1 and %d days", MaxDownloadHistoryRetentionDays)

// DownloadHistoryRetention is how long PicoShare keeps records of file
// downloads. The zero value keeps download history forever.
type DownloadHistoryRetention struct {
	days uint16
}

// KeepDownloadHistoryForever is a retention period that never deletes
// download history.
var KeepDownloadHistoryForever = DownloadHistoryRetention{}

func NewDownloadHistoryRetentionInDays(days uint16) (DownloadHistoryRetention, error) {
	if days < 1 || days > MaxDownloadHistoryRetentionDays {
		return DownloadHistoryRetention{}, ErrInvalidDownloadHistoryRetention
	}
	return DownloadHistoryRetention{days: days}, nil
}

// IsForever reports whether PicoShare keeps download history indefinitely.
func (r DownloadHistoryRetention) IsForever() bool {
	return r.days == 0
}

func (r DownloadHistoryRetention) Days() uint16 {
	if r.IsForever() {
		panic("cannot access days of a download history retention that keeps history forever")
	}
	return r.days
}

// CutoffFrom returns the time before which download records are old enough to
// delete.
func (r DownloadHistoryRetention) CutoffFrom(now time.Time) time.Time {
	return now.Add(-time.Duration(r.Days()) * hoursPerDay * time.Hour)
}

func (r DownloadHistoryRetention) String() string {
	if r.IsForever() {
		return "forever"
	}
	return fmt.Sprintf("%d days", r.days)
}
