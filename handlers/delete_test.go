package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mtlynch/picoshare/garbagecollect"
	"github.com/mtlynch/picoshare/handlers"
	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
	"github.com/mtlynch/picoshare/store/test_sqlite"
)

var nilSpaceCheckFunc handlers.SpaceCheckFunc
var nilGarbageCollector *garbagecollect.Collector

func TestDeleteExistingFile(t *testing.T) {
	dataStore := test_sqlite.New(t)
	now := mustParseTime("2023-01-01T00:00:00Z")
	loginCookie := mustLoginAsAdmin(t, &dataStore, now)
	fileContents := "dummy data"
	dataStore.InsertEntry(strings.NewReader(fileContents),
		picoshare.UploadMetadata{
			ID:       picoshare.MustCreateEntryID("hR87apiUCj"),
			Uploaded: now,
			Expires:  mustParseExpirationTime("2024-01-01T00:00:00Z"),
			Size:     mustParseFileSize(len(fileContents)),
		})
	s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

	req := httptest.NewRequest(http.MethodDelete, "/api/entry/hR87apiUCj", nil)
	req.AddCookie(loginCookie)

	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	res := rec.Result()

	if status := res.StatusCode; status != http.StatusOK {
		t.Fatalf("DELETE /api/entry returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	_, err := dataStore.GetEntryMetadata(picoshare.MustCreateEntryID("hR87apiUCj"))
	if _, ok := err.(store.EntryNotFoundError); !ok {
		t.Fatalf("expected entry %v to be deleted", picoshare.MustCreateEntryID("hR87apiUCj"))
	}
}

func TestDeleteNonExistentFile(t *testing.T) {
	dataStore := test_sqlite.New(t)
	now := mustParseTime("2023-01-01T00:00:00Z")
	loginCookie := mustLoginAsAdmin(t, &dataStore, now)
	s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

	req := httptest.NewRequest(http.MethodDelete, "/api/entry/hR87apiUCj", nil)
	req.AddCookie(loginCookie)

	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	res := rec.Result()

	// A missing entry gets 404, the same response as an entry that exists but
	// belongs to someone else, so a download link's existence isn't
	// distinguishable from a stranger's ID guess.
	if status := res.StatusCode; status != http.StatusNotFound {
		t.Fatalf("DELETE /api/entry returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}

func TestDeleteInvalidEntryID(t *testing.T) {
	dataStore := test_sqlite.New(t)
	now := mustParseTime("2023-01-01T00:00:00Z")
	loginCookie := mustLoginAsAdmin(t, &dataStore, now)
	s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

	req := httptest.NewRequest(http.MethodDelete, "/api/entry/invalid-entry-id", nil)
	req.AddCookie(loginCookie)

	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	res := rec.Result()

	if status := res.StatusCode; status != http.StatusBadRequest {
		t.Fatalf("DELETE /api/entry returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}
