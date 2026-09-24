package sqlite_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
	"github.com/mtlynch/picoshare/store/test_sqlite"
)

func TestRecordUserLoginFirstUserBecomesAdmin(t *testing.T) {
	dataStore := test_sqlite.New(t)

	alice, err := dataStore.RecordUserLogin(mustIdentity(t, "alice-sub", "alice", "alice@example.com"))
	if err != nil {
		t.Fatalf("failed to record login: %v", err)
	}
	if got, want := alice.IsAdmin, true; got != want {
		t.Errorf("first user IsAdmin=%v, want %v", got, want)
	}

	bob, err := dataStore.RecordUserLogin(mustIdentity(t, "bob-sub", "bob", "bob@example.com"))
	if err != nil {
		t.Fatalf("failed to record login: %v", err)
	}
	if got, want := bob.IsAdmin, false; got != want {
		t.Errorf("second user IsAdmin=%v, want %v", got, want)
	}
}

func TestRecordUserLoginUpdatesProfileOnReturningLogin(t *testing.T) {
	dataStore := test_sqlite.New(t)

	first, err := dataStore.RecordUserLogin(mustIdentity(t, "alice-sub", "alice", "alice@example.com"))
	if err != nil {
		t.Fatalf("failed to record login: %v", err)
	}

	second, err := dataStore.RecordUserLogin(mustIdentity(t, "alice-sub", "alice2", "alice2@example.com"))
	if err != nil {
		t.Fatalf("failed to record login: %v", err)
	}

	if got, want := second.ID, first.ID; got != want {
		t.Errorf("returning login changed user ID: got %v, want %v", got, want)
	}
	if got, want := second.Username.String(), "alice2"; got != want {
		t.Errorf("Username=%s, want %s", got, want)
	}
	if got, want := second.Email.String(), "alice2@example.com"; got != want {
		t.Errorf("Email=%s, want %s", got, want)
	}
	if got, want := second.IsAdmin, true; got != want {
		t.Errorf("returning login changed IsAdmin: got %v, want %v", got, want)
	}
}

func TestRecordUserLoginFirstAdminClaimsOwnerlessRows(t *testing.T) {
	dataStore := test_sqlite.New(t)

	d := "dummy data"
	entryID := picoshare.MustCreateEntryID("AAAAAAAAAA")
	if err := dataStore.InsertEntry(strings.NewReader(d), picoshare.UploadMetadata{
		ID:       entryID,
		Uploaded: mustParseTime("2023-01-01T00:00:00Z"),
		Expires:  picoshare.NeverExpire,
		Size:     mustParseFileSize(len(d)),
	}); err != nil {
		t.Fatalf("failed to insert entry: %v", err)
	}

	glID := picoshare.GuestLinkID("abcdefgh23456789")
	if err := dataStore.InsertGuestLink(picoshare.GuestLink{
		ID:              glID,
		Created:         mustParseTime("2023-01-01T00:00:00Z"),
		UrlExpires:      picoshare.NeverExpire,
		MaxFileLifetime: picoshare.FileLifetimeInfinite,
		MaxFileBytes:    picoshare.GuestUploadUnlimitedFileSize,
		MaxFileUploads:  picoshare.GuestUploadUnlimitedFileUploads,
	}); err != nil {
		t.Fatalf("failed to insert guest link: %v", err)
	}

	admin, err := dataStore.RecordUserLogin(mustIdentity(t, "alice-sub", "alice", "alice@example.com"))
	if err != nil {
		t.Fatalf("failed to record login: %v", err)
	}

	entry, err := dataStore.GetEntryMetadata(entryID)
	if err != nil {
		t.Fatalf("failed to read entry: %v", err)
	}
	if got, want := entry.OwnerID, admin.ID; got != want {
		t.Errorf("legacy entry OwnerID=%v, want %v", got, want)
	}

	gl, err := dataStore.GetGuestLink(glID)
	if err != nil {
		t.Fatalf("failed to read guest link: %v", err)
	}
	if got, want := gl.OwnerID, admin.ID; got != want {
		t.Errorf("legacy guest link OwnerID=%v, want %v", got, want)
	}
}

func TestGrantAndRevokeAdmin(t *testing.T) {
	dataStore := test_sqlite.New(t)

	admin, err := dataStore.RecordUserLogin(mustIdentity(t, "alice-sub", "alice", "alice@example.com"))
	if err != nil {
		t.Fatalf("failed to record login: %v", err)
	}
	other, err := dataStore.RecordUserLogin(mustIdentity(t, "bob-sub", "bob", "bob@example.com"))
	if err != nil {
		t.Fatalf("failed to record login: %v", err)
	}

	if err := dataStore.RevokeAdmin(admin.ID); !isLastAdminError(err) {
		t.Fatalf("revoking the last admin should fail with LastAdminError, got %v", err)
	}

	if err := dataStore.GrantAdmin(other.ID); err != nil {
		t.Fatalf("failed to grant admin: %v", err)
	}
	if err := dataStore.RevokeAdmin(admin.ID); err != nil {
		t.Fatalf("failed to revoke admin now that there are two admins: %v", err)
	}

	users, err := dataStore.GetUsers()
	if err != nil {
		t.Fatalf("failed to list users: %v", err)
	}
	adminCount := 0
	for _, u := range users {
		if u.IsAdmin {
			adminCount++
		}
	}
	if got, want := adminCount, 1; got != want {
		t.Errorf("admin count=%d, want %d", got, want)
	}
}

func TestRevokeAdminOfUnknownUser(t *testing.T) {
	dataStore := test_sqlite.New(t)
	if _, err := dataStore.RecordUserLogin(mustIdentity(t, "alice-sub", "alice", "alice@example.com")); err != nil {
		t.Fatalf("failed to record login: %v", err)
	}

	err := dataStore.RevokeAdmin(picoshare.UserIDFromInt64(999))
	var notFound store.UserNotFoundError
	if !errors.As(err, &notFound) {
		t.Errorf("RevokeAdmin of an unknown user = %v, want UserNotFoundError", err)
	}
}

func mustIdentity(t *testing.T, subject, username, email string) picoshare.UserIdentity {
	t.Helper()
	s, err := picoshare.NewOIDCSubject(subject)
	if err != nil {
		t.Fatalf("failed to create subject: %v", err)
	}
	u, err := picoshare.NewUsername(username)
	if err != nil {
		t.Fatalf("failed to create username: %v", err)
	}
	e, err := picoshare.NewEmailAddress(email)
	if err != nil {
		t.Fatalf("failed to create email: %v", err)
	}
	return picoshare.UserIdentity{Subject: s, Username: u, Email: e}
}

func isLastAdminError(err error) bool {
	var lastAdmin store.LastAdminError
	return errors.As(err, &lastAdmin)
}
