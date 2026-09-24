package garbagecollect_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-test/deep"

	"github.com/mtlynch/picoshare/garbagecollect"
	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store/test_sqlite"
)

func TestCollectDoesNothingWhenStoreIsEmpty(t *testing.T) {
	dataStore := test_sqlite.New(t)
	c := garbagecollect.NewCollector(dataStore, fixedNow)
	err := c.Collect()
	if err != nil {
		t.Fatalf("garbage collection failed: %v", err)
	}

	remaining, err := dataStore.GetEntriesMetadata()
	if err != nil {
		t.Fatalf("retrieving datastore metadata failed: %v", err)
	}

	expected := []picoshare.UploadMetadata{}
	if !reflect.DeepEqual(expected, remaining) {
		t.Fatalf("unexpected results in datastore: got %+v, want %+v", remaining, expected)
	}
}

func TestCollectExpiredFile(t *testing.T) {
	dataStore := test_sqlite.New(t)
	d := "dummy data"
	expireInFiveMins := mustParseExpirationTime("2025-01-01T00:05:00Z")
	dataStore.InsertEntry(strings.NewReader(d),
		picoshare.UploadMetadata{
			ID:       picoshare.MustCreateEntryID("AAAAAAAAAA"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  mustParseExpirationTime("2024-01-01T00:00:00Z"),
			Size:     mustParseFileSize(len(d)),
		})
	dataStore.InsertEntryDownload(
		picoshare.MustCreateEntryID("AAAAAAAAAA"),
		picoshare.DownloadRecord{
			Time:      mustParseTime("2023-06-01T12:00:00Z"),
			ClientIP:  "192.168.1.1",
			UserAgent: "test-agent",
		})
	dataStore.InsertEntry(strings.NewReader(d),
		picoshare.UploadMetadata{
			ID:       picoshare.MustCreateEntryID("BBBBBBBBBB"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  mustParseExpirationTime("3000-01-01T00:00:00Z"),
			Size:     mustParseFileSize(len(d)),
		})
	dataStore.InsertEntry(strings.NewReader(d),
		picoshare.UploadMetadata{
			ID:       picoshare.MustCreateEntryID("CCCCCCCCCC"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  picoshare.NeverExpire,
			Size:     mustParseFileSize(len(d)),
		})
	dataStore.InsertEntry(strings.NewReader(d),
		picoshare.UploadMetadata{
			ID:       picoshare.MustCreateEntryID("DDDDDDDDDD"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  mustParseExpirationTime("2024-12-31T23:59:59Z"),
			Size:     mustParseFileSize(len(d)),
		})
	dataStore.InsertEntry(strings.NewReader(d),
		picoshare.UploadMetadata{
			ID:       picoshare.MustCreateEntryID("EEEEEEEEEE"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  expireInFiveMins,
			Size:     mustParseFileSize(len(d)),
		})

	c := garbagecollect.NewCollector(dataStore, fixedNow)
	err := c.Collect()
	if err != nil {
		t.Fatalf("garbage collection failed: %v", err)
	}

	remaining, err := dataStore.GetEntriesMetadata()
	if err != nil {
		t.Fatalf("retrieving datastore metadata failed: %v", err)
	}

	expected := []picoshare.UploadMetadata{
		{
			ID:       picoshare.MustCreateEntryID("BBBBBBBBBB"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  mustParseExpirationTime("3000-01-01T00:00:00Z"),
			Size:     mustParseFileSize(len(d)),
		},
		{
			ID:       picoshare.MustCreateEntryID("CCCCCCCCCC"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  picoshare.NeverExpire,
			Size:     mustParseFileSize(len(d)),
		},
		{
			ID:       picoshare.MustCreateEntryID("EEEEEEEEEE"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  expireInFiveMins,
			Size:     mustParseFileSize(len(d)),
		},
	}
	if diff := deep.Equal(expected, remaining); diff != nil {
		t.Errorf("unexpected results in datastore: got %v, want %v, diff = %v", remaining, expected, diff)
		t.Errorf("got=%+v", remaining)
		t.Errorf("want=%+v", expected)
		t.Errorf("diff=%+v", diff)
		t.FailNow()
	}
}

func TestCollectDoesNothingWhenNoFilesAreExpired(t *testing.T) {
	dataStore := test_sqlite.New(t)
	d := "dummy data"
	dataStore.InsertEntry(strings.NewReader(d),
		picoshare.UploadMetadata{
			ID:       picoshare.MustCreateEntryID("AAAAAAAAAA"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  mustParseExpirationTime("4000-01-01T00:00:00Z"),
			Size:     mustParseFileSize(len(d)),
		})
	dataStore.InsertEntry(strings.NewReader(d),
		picoshare.UploadMetadata{
			ID:       picoshare.MustCreateEntryID("BBBBBBBBBB"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  mustParseExpirationTime("3000-01-01T00:00:00Z"),
			Size:     mustParseFileSize(len(d)),
		})
	dataStore.InsertEntry(strings.NewReader(d),
		picoshare.UploadMetadata{
			ID:       picoshare.MustCreateEntryID("CCCCCCCCCC"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  picoshare.NeverExpire,
			Size:     mustParseFileSize(len(d)),
		})

	c := garbagecollect.NewCollector(dataStore, fixedNow)
	err := c.Collect()
	if err != nil {
		t.Fatalf("garbage collection failed: %v", err)
	}

	remaining, err := dataStore.GetEntriesMetadata()
	if err != nil {
		t.Fatalf("retrieving datastore metadata failed: %v", err)
	}

	// Sort the elements so they have a consistent ordering.
	sort.Slice(remaining, func(i, j int) bool {
		return (time.Time(remaining[i].Expires)).After(time.Time(remaining[j].Expires))
	})

	expected := []picoshare.UploadMetadata{
		{
			ID:       picoshare.MustCreateEntryID("AAAAAAAAAA"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  mustParseExpirationTime("4000-01-01T00:00:00Z"),
			Size:     mustParseFileSize(len(d)),
		},
		{
			ID:       picoshare.MustCreateEntryID("BBBBBBBBBB"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  mustParseExpirationTime("3000-01-01T00:00:00Z"),
			Size:     mustParseFileSize(len(d)),
		},
		{
			ID:       picoshare.MustCreateEntryID("CCCCCCCCCC"),
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  picoshare.NeverExpire,
			Size:     mustParseFileSize(len(d)),
		},
	}

	if diff := deep.Equal(expected, remaining); diff != nil {
		t.Errorf("unexpected results in datastore: got %v, want %v, diff = %v", remaining, expected, diff)
		t.Errorf("got=%+v", remaining)
		t.Errorf("want=%+v", expected)
		t.Errorf("diff=%+v", diff)
		t.FailNow()
	}
}

func TestCollectDeletesExpiredDownloadHistory(t *testing.T) {
	for _, tt := range []struct {
		explanation       string
		retention         picoshare.DownloadHistoryRetention
		downloadsExpected []picoshare.DownloadRecord
	}{
		{
			explanation: "keeping history forever retains all downloads",
			retention:   picoshare.KeepDownloadHistoryForever,
			downloadsExpected: []picoshare.DownloadRecord{
				{
					Time:      mustParseTime("2024-12-25T00:00:00Z"),
					ClientIP:  "10.0.0.2",
					UserAgent: "dummy-agent",
				},
				{
					Time:      mustParseTime("2024-11-01T00:00:00Z"),
					ClientIP:  "10.0.0.1",
					UserAgent: "dummy-agent",
				},
			},
		},
		{
			explanation: "30-day retention deletes downloads older than 30 days",
			retention:   mustCreateDownloadHistoryRetention(30),
			downloadsExpected: []picoshare.DownloadRecord{
				{
					Time:      mustParseTime("2024-12-25T00:00:00Z"),
					ClientIP:  "10.0.0.2",
					UserAgent: "dummy-agent",
				},
			},
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			dataStore := test_sqlite.New(t)
			d := "dummy data"
			id := picoshare.MustCreateEntryID("AAAAAAAAAA")
			if err := dataStore.InsertEntry(strings.NewReader(d),
				picoshare.UploadMetadata{
					ID:       id,
					Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
					Expires:  picoshare.NeverExpire,
					Size:     mustParseFileSize(len(d)),
				}); err != nil {
				t.Fatalf("failed to insert entry: %v", err)
			}
			for _, r := range []picoshare.DownloadRecord{
				{
					Time:      mustParseTime("2024-11-01T00:00:00Z"),
					ClientIP:  "10.0.0.1",
					UserAgent: "dummy-agent",
				},
				{
					Time:      mustParseTime("2024-12-25T00:00:00Z"),
					ClientIP:  "10.0.0.2",
					UserAgent: "dummy-agent",
				},
			} {
				if err := dataStore.InsertEntryDownload(id, r); err != nil {
					t.Fatalf("failed to insert download: %v", err)
				}
			}
			if err := dataStore.UpdateSettings(picoshare.Settings{
				DefaultFileLifetime:      picoshare.NewFileLifetimeInDays(30),
				DownloadHistoryRetention: tt.retention,
			}); err != nil {
				t.Fatalf("failed to update settings: %v", err)
			}

			c := garbagecollect.NewCollector(dataStore, fixedNow)
			if err := c.Collect(); err != nil {
				t.Fatalf("garbage collection failed: %v", err)
			}

			downloads, err := dataStore.GetEntryDownloads(id)
			if err != nil {
				t.Fatalf("failed to read downloads: %v", err)
			}
			if diff := deep.Equal(tt.downloadsExpected, downloads); diff != nil {
				t.Errorf("unexpected downloads after collection: %v", diff)
			}
		})
	}
}

func TestCollectReturnsFreeSpaceToFilesystem(t *testing.T) {
	dataStore := test_sqlite.NewWithChunkSize(t, 4096)
	d := strings.Repeat("A", 100*4096)
	id := picoshare.MustCreateEntryID("AAAAAAAAAA")
	if err := dataStore.InsertEntry(strings.NewReader(d),
		picoshare.UploadMetadata{
			ID:       id,
			Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
			Expires:  picoshare.NeverExpire,
			Size:     mustParseFileSize(len(d)),
		}); err != nil {
		t.Fatalf("failed to insert entry: %v", err)
	}
	if err := dataStore.DeleteEntry(id); err != nil {
		t.Fatalf("failed to delete entry: %v", err)
	}

	reclaimableBefore, err := dataStore.ReclaimableBytes()
	if err != nil {
		t.Fatalf("failed to measure reclaimable bytes: %v", err)
	}
	if reclaimableBefore < uint64(len(d)) {
		t.Fatalf("reclaimable bytes before collection=%d, want at least %d", reclaimableBefore, len(d))
	}

	c := garbagecollect.NewCollector(dataStore, fixedNow)
	if err := c.Collect(); err != nil {
		t.Fatalf("garbage collection failed: %v", err)
	}

	reclaimableAfter, err := dataStore.ReclaimableBytes()
	if err != nil {
		t.Fatalf("failed to measure reclaimable bytes: %v", err)
	}
	if got, want := reclaimableAfter, uint64(0); got != want {
		t.Errorf("reclaimable bytes after collection=%d, want %d", got, want)
	}
}

func TestCollectRecordsLastRun(t *testing.T) {
	dataStore := test_sqlite.New(t)
	c := garbagecollect.NewCollector(dataStore, fixedNow)

	if got, want := c.LastRun(), (garbagecollect.CollectionResult{}); got != want {
		t.Fatalf("last run before collection=%+v, want %+v", got, want)
	}

	if err := c.Collect(); err != nil {
		t.Fatalf("garbage collection failed: %v", err)
	}

	if got, want := c.LastRun(), (garbagecollect.CollectionResult{Time: fixedNow()}); got != want {
		t.Errorf("last run after collection=%+v, want %+v", got, want)
	}
}

func fixedNow() time.Time {
	return mustParseTime("2025-01-01T00:00:00Z")
}

func mustCreateDownloadHistoryRetention(days uint16) picoshare.DownloadHistoryRetention {
	r, err := picoshare.NewDownloadHistoryRetentionInDays(days)
	if err != nil {
		panic(err)
	}
	return r
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func mustParseExpirationTime(s string) picoshare.ExpirationTime {
	et, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return picoshare.ExpirationTime(et)
}

func mustParseFileSize(val int) picoshare.FileSize {
	fileSize, err := picoshare.FileSizeFromInt(val)
	if err != nil {
		panic(err)
	}

	return fileSize
}
