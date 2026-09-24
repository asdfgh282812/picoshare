package picoshare_test

import (
	"testing"
	"time"

	"github.com/mtlynch/picoshare/picoshare"
)

func TestNewDownloadHistoryRetentionInDays(t *testing.T) {
	for _, tt := range []struct {
		explanation     string
		days            uint16
		isValidExpected bool
	}{
		{
			explanation:     "zero days is invalid",
			days:            0,
			isValidExpected: false,
		},
		{
			explanation:     "one day is valid",
			days:            1,
			isValidExpected: true,
		},
		{
			explanation:     "the maximum number of days is valid",
			days:            picoshare.MaxDownloadHistoryRetentionDays,
			isValidExpected: true,
		},
		{
			explanation:     "one day more than the maximum is invalid",
			days:            picoshare.MaxDownloadHistoryRetentionDays + 1,
			isValidExpected: false,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			retention, err := picoshare.NewDownloadHistoryRetentionInDays(tt.days)
			isValid := err == nil
			if got, want := isValid, tt.isValidExpected; got != want {
				t.Fatalf("isValid=%v, want %v (err=%v)", got, want, err)
			}
			if !isValid {
				return
			}
			if got, want := retention.Days(), tt.days; got != want {
				t.Errorf("days=%d, want %d", got, want)
			}
		})
	}
}

func TestDownloadHistoryRetentionCutoffFrom(t *testing.T) {
	retention, err := picoshare.NewDownloadHistoryRetentionInDays(30)
	if err != nil {
		t.Fatalf("failed to create retention: %v", err)
	}
	now := time.Date(2025, time.March, 31, 12, 0, 0, 0, time.UTC)

	if got, want := retention.CutoffFrom(now), time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("cutoff=%v, want %v", got, want)
	}
}

func TestKeepDownloadHistoryForeverIsZeroValue(t *testing.T) {
	if got, want := (picoshare.DownloadHistoryRetention{}).IsForever(), true; got != want {
		t.Errorf("IsForever()=%v, want %v", got, want)
	}
}
