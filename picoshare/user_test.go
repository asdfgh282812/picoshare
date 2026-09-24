package picoshare_test

import (
	"strings"
	"testing"

	"github.com/mtlynch/picoshare/picoshare"
)

func TestUserIDFromString(t *testing.T) {
	for _, tt := range []struct {
		explanation     string
		input           string
		isValidExpected bool
	}{
		{
			explanation:     "a positive integer is valid",
			input:           "5",
			isValidExpected: true,
		},
		{
			explanation:     "zero is invalid",
			input:           "0",
			isValidExpected: false,
		},
		{
			explanation:     "a negative integer is invalid",
			input:           "-1",
			isValidExpected: false,
		},
		{
			explanation:     "non-numeric text is invalid",
			input:           "banana",
			isValidExpected: false,
		},
		{
			explanation:     "empty text is invalid",
			input:           "",
			isValidExpected: false,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			id, err := picoshare.UserIDFromString(tt.input)
			isValid := err == nil
			if got, want := isValid, tt.isValidExpected; got != want {
				t.Fatalf("isValid=%v, want %v (err=%v)", got, want, err)
			}
			if !isValid {
				return
			}
			if got, want := id.String(), tt.input; got != want {
				t.Errorf("String()=%s, want %s", got, want)
			}
		})
	}
}

func TestUserIDZeroValueIsEmpty(t *testing.T) {
	if got, want := (picoshare.UserID{}).Empty(), true; got != want {
		t.Errorf("Empty()=%v, want %v", got, want)
	}
	if got, want := picoshare.UserIDFromInt64(1).Empty(), false; got != want {
		t.Errorf("Empty()=%v, want %v", got, want)
	}
}

func TestNewOIDCSubject(t *testing.T) {
	for _, tt := range []struct {
		explanation     string
		input           string
		isValidExpected bool
	}{
		{
			explanation:     "empty subject is invalid",
			input:           "",
			isValidExpected: false,
		},
		{
			explanation:     "a typical subject is valid",
			input:           "1234567890",
			isValidExpected: true,
		},
		{
			explanation:     "the maximum length is valid",
			input:           strings.Repeat("a", picoshare.MaxOIDCSubjectLength),
			isValidExpected: true,
		},
		{
			explanation:     "one character beyond the maximum length is invalid",
			input:           strings.Repeat("a", picoshare.MaxOIDCSubjectLength+1),
			isValidExpected: false,
		},
		{
			explanation:     "NUL bytes are rejected",
			input:           "abc\x00def",
			isValidExpected: false,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			_, err := picoshare.NewOIDCSubject(tt.input)
			isValid := err == nil
			if got, want := isValid, tt.isValidExpected; got != want {
				t.Fatalf("isValid=%v, want %v (err=%v)", got, want, err)
			}
		})
	}
}

func TestNewUsername(t *testing.T) {
	for _, tt := range []struct {
		explanation     string
		input           string
		isValidExpected bool
	}{
		{
			explanation:     "empty username is invalid",
			input:           "",
			isValidExpected: false,
		},
		{
			explanation:     "a typical username is valid",
			input:           "alice",
			isValidExpected: true,
		},
		{
			explanation:     "the maximum length is valid",
			input:           strings.Repeat("a", picoshare.MaxUsernameLength),
			isValidExpected: true,
		},
		{
			explanation:     "one character beyond the maximum length is invalid",
			input:           strings.Repeat("a", picoshare.MaxUsernameLength+1),
			isValidExpected: false,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			_, err := picoshare.NewUsername(tt.input)
			isValid := err == nil
			if got, want := isValid, tt.isValidExpected; got != want {
				t.Fatalf("isValid=%v, want %v (err=%v)", got, want, err)
			}
		})
	}
}

func TestNewEmailAddress(t *testing.T) {
	for _, tt := range []struct {
		explanation     string
		input           string
		isValidExpected bool
	}{
		{
			explanation:     "a typical email address is valid",
			input:           "alice@example.com",
			isValidExpected: true,
		},
		{
			explanation:     "text without an '@' is invalid",
			input:           "alice",
			isValidExpected: false,
		},
		{
			explanation:     "empty text is invalid",
			input:           "",
			isValidExpected: false,
		},
		{
			explanation:     "text longer than the maximum is invalid",
			input:           strings.Repeat("a", picoshare.MaxEmailAddressBytes) + "@example.com",
			isValidExpected: false,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			_, err := picoshare.NewEmailAddress(tt.input)
			isValid := err == nil
			if got, want := isValid, tt.isValidExpected; got != want {
				t.Fatalf("isValid=%v, want %v (err=%v)", got, want, err)
			}
		})
	}
}

func TestEmailAddressZeroValueIsEmpty(t *testing.T) {
	if got, want := picoshare.NoEmailAddress.Empty(), true; got != want {
		t.Errorf("Empty()=%v, want %v", got, want)
	}
}

func TestUserCanManageEntry(t *testing.T) {
	alice := picoshare.User{ID: picoshare.UserIDFromInt64(1)}
	bob := picoshare.User{ID: picoshare.UserIDFromInt64(2)}
	admin := picoshare.User{ID: picoshare.UserIDFromInt64(3), IsAdmin: true}
	aliceEntry := picoshare.UploadMetadata{OwnerID: alice.ID}
	legacyEntry := picoshare.UploadMetadata{}

	for _, tt := range []struct {
		explanation string
		user        picoshare.User
		entry       picoshare.UploadMetadata
		canManage   bool
	}{
		{"owner can manage their own entry", alice, aliceEntry, true},
		{"non-owner cannot manage another user's entry", bob, aliceEntry, false},
		{"admin can manage any entry", admin, aliceEntry, true},
		{"non-admin cannot manage an unowned legacy entry", bob, legacyEntry, false},
		{"admin can manage an unowned legacy entry", admin, legacyEntry, true},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			if got, want := tt.user.CanManageEntry(tt.entry), tt.canManage; got != want {
				t.Errorf("CanManageEntry()=%v, want %v", got, want)
			}
		})
	}
}

func TestUserOwnsGuestLink(t *testing.T) {
	alice := picoshare.User{ID: picoshare.UserIDFromInt64(1)}
	bob := picoshare.User{ID: picoshare.UserIDFromInt64(2)}
	aliceLink := picoshare.GuestLink{OwnerID: alice.ID}

	if got, want := alice.OwnsGuestLink(aliceLink), true; got != want {
		t.Errorf("OwnsGuestLink()=%v, want %v", got, want)
	}
	if got, want := bob.OwnsGuestLink(aliceLink), false; got != want {
		t.Errorf("OwnsGuestLink()=%v, want %v", got, want)
	}
}
