package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mtlynch/picoshare/handlers"
	"github.com/mtlynch/picoshare/store/test_sqlite"
)

func TestMemberGrantAdminPut(t *testing.T) {
	dataStore := test_sqlite.New(t)
	now := mustParseTime("2024-01-01T00:00:00Z")
	_, adminCookie := mustLoginAsUser(t, &dataStore, "admin", now)
	nonAdmin, _ := mustLoginAsUser(t, &dataStore, "alice", now)
	s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/users/%s/grant-admin", nonAdmin.ID), nil)
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	res := rec.Result()

	if got, want := res.StatusCode, http.StatusNoContent; got != want {
		t.Fatalf("PUT grant-admin returned status %d, want %d", got, want)
	}

	users, err := dataStore.GetUsers()
	if err != nil {
		t.Fatalf("failed to read back users: %v", err)
	}
	var got bool
	for _, u := range users {
		if u.ID == nonAdmin.ID {
			got = u.IsAdmin
		}
	}
	if !got {
		t.Error("user was not promoted to administrator")
	}
}

func TestMemberGrantAdminPutRejectsUnknownUser(t *testing.T) {
	dataStore := test_sqlite.New(t)
	_, adminCookie := mustLoginAsUser(t, &dataStore, "admin", mustParseTime("2024-01-01T00:00:00Z"))
	s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/999/grant-admin", nil)
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	res := rec.Result()

	if got, want := res.StatusCode, http.StatusNotFound; got != want {
		t.Fatalf("PUT grant-admin for unknown user returned status %d, want %d", got, want)
	}
}

func TestMemberRevokeAdminPut(t *testing.T) {
	dataStore := test_sqlite.New(t)
	now := mustParseTime("2024-01-01T00:00:00Z")
	_, adminCookie := mustLoginAsUser(t, &dataStore, "admin", now)
	other, _ := mustLoginAsUser(t, &dataStore, "bob", now)
	if err := dataStore.GrantAdmin(other.ID); err != nil {
		t.Fatalf("failed to grant admin to second user: %v", err)
	}
	s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/users/%s/revoke-admin", other.ID), nil)
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	res := rec.Result()

	if got, want := res.StatusCode, http.StatusNoContent; got != want {
		t.Fatalf("PUT revoke-admin returned status %d, want %d", got, want)
	}

	users, err := dataStore.GetUsers()
	if err != nil {
		t.Fatalf("failed to read back users: %v", err)
	}
	for _, u := range users {
		if u.ID == other.ID && u.IsAdmin {
			t.Error("user still has administrator access after being revoked")
		}
	}
}

func TestMemberRevokeAdminPutRejectsSelfDemotion(t *testing.T) {
	dataStore := test_sqlite.New(t)
	now := mustParseTime("2024-01-01T00:00:00Z")
	admin, adminCookie := mustLoginAsUser(t, &dataStore, "admin", now)
	other, _ := mustLoginAsUser(t, &dataStore, "bob", now)
	if err := dataStore.GrantAdmin(other.ID); err != nil {
		t.Fatalf("failed to grant admin to second user: %v", err)
	}
	s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/users/%s/revoke-admin", admin.ID), nil)
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	res := rec.Result()

	if got, want := res.StatusCode, http.StatusBadRequest; got != want {
		t.Fatalf("PUT revoke-admin on self returned status %d, want %d", got, want)
	}

	users, err := dataStore.GetUsers()
	if err != nil {
		t.Fatalf("failed to read back users: %v", err)
	}
	for _, u := range users {
		if u.ID == admin.ID && !u.IsAdmin {
			t.Error("self-revoke request should not have changed the admin's own status")
		}
	}
}

func TestMemberRevokeAdminPutRejectsUnknownUser(t *testing.T) {
	dataStore := test_sqlite.New(t)
	_, adminCookie := mustLoginAsUser(t, &dataStore, "admin", mustParseTime("2024-01-01T00:00:00Z"))
	s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/999/revoke-admin", nil)
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	res := rec.Result()

	if got, want := res.StatusCode, http.StatusNotFound; got != want {
		t.Fatalf("PUT revoke-admin for unknown user returned status %d, want %d", got, want)
	}
}
