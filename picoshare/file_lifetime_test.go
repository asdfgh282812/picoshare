package picoshare_test

import (
	"testing"

	"github.com/mtlynch/picoshare/picoshare"
)

func TestFileLifetime(t *testing.T) {
	for _, tt := range []struct {
		explanation      string
		lifetime         picoshare.FileLifetime
		days             uint16
		years            uint16
		isOnYearBoundary bool
	}{
		{
			explanation:      "1 day",
			lifetime:         picoshare.NewFileLifetimeInDays(1),
			days:             1,
			years:            0,
			isOnYearBoundary: false,
		},
		{
			explanation:      "7 days",
			lifetime:         picoshare.NewFileLifetimeInDays(7),
			days:             7,
			years:            0,
			isOnYearBoundary: false,
		},
		{
			explanation:      "30 days",
			lifetime:         picoshare.NewFileLifetimeInDays(30),
			days:             30,
			years:            0,
			isOnYearBoundary: false,
		},
		{
			explanation:      "1 year",
			lifetime:         picoshare.NewFileLifetimeInYears(1),
			days:             365,
			years:            1,
			isOnYearBoundary: true,
		},
		{
			explanation:      "366 days",
			lifetime:         picoshare.NewFileLifetimeInDays(366),
			days:             366,
			years:            1,
			isOnYearBoundary: false,
		},
		{
			explanation:      "10 years",
			lifetime:         picoshare.NewFileLifetimeInYears(10),
			days:             3650,
			years:            10,
			isOnYearBoundary: true,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			if got, want := tt.lifetime.Days(), tt.days; got != want {
				t.Errorf("days=%v, want=%v", got, want)
			}
			if got, want := tt.lifetime.Years(), tt.years; got != want {
				t.Errorf("years=%v, want=%v", got, want)
			}
			if got, want := tt.lifetime.IsYearBoundary(), tt.isOnYearBoundary; got != want {
				t.Errorf("isOnYearBoundary=%v, want=%v", got, want)
			}
		})
	}
}
