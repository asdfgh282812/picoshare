package picoshare

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxOIDCSubjectLength = 255
	MaxUsernameLength    = 200
	MaxEmailAddressBytes = 320
)

var (
	ErrInvalidUserID       = fmt.Errorf("user ID must be a positive integer")
	ErrInvalidOIDCSubject  = fmt.Errorf("OIDC subject must contain between 1 and %d Unicode code points", MaxOIDCSubjectLength)
	ErrInvalidUsername     = fmt.Errorf("username must contain between 1 and %d Unicode code points", MaxUsernameLength)
	ErrInvalidEmailAddress = fmt.Errorf("email address must contain '@' and be at most %d bytes", MaxEmailAddressBytes)
)

type (
	// UserID identifies a PicoShare user. The zero value means "no owner,"
	// which only ever applies to entries and guest links created before
	// PicoShare supported multiple users.
	UserID struct {
		value int64
	}

	// OIDCSubject is the "sub" claim an identity provider asserts for a user.
	// It's the durable identifier PicoShare uses to recognize a returning user.
	OIDCSubject struct {
		value string
	}

	// Username is the display name an identity provider asserts for a user.
	Username struct {
		value string
	}

	// EmailAddress is a user's email address. The zero value means the
	// identity provider didn't supply one.
	EmailAddress struct {
		value string
	}

	// UserIdentity is the set of verified claims an identity provider asserts
	// about a user after a successful login.
	UserIdentity struct {
		Subject  OIDCSubject
		Username Username
		Email    EmailAddress
	}

	User struct {
		ID       UserID
		Subject  OIDCSubject
		Username Username
		Email    EmailAddress
		IsAdmin  bool
		// PreferredLanguage is the user's chosen interface language. The zero
		// value means the user hasn't chosen one, so PicoShare falls back to
		// their language cookie or the site default.
		PreferredLanguage Language
		Created           time.Time
		LastLogin         time.Time
	}
)

// UserIDFromInt64 constructs a user ID from a trusted source, such as a
// database row. Use UserIDFromString for user-provided text.
func UserIDFromInt64(v int64) UserID {
	return UserID{value: v}
}

// UserIDFromString constructs a user ID from user-provided text, such as a
// URL path segment.
func UserIDFromString(raw string) (UserID, error) {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 1 {
		return UserID{}, ErrInvalidUserID
	}
	return UserID{value: v}, nil
}

// Empty reports whether the user ID is the zero value, meaning "no owner."
func (id UserID) Empty() bool {
	return id.value == 0
}

func (id UserID) Int64() int64 {
	return id.value
}

func (id UserID) String() string {
	return strconv.FormatInt(id.value, 10)
}

// NewOIDCSubject constructs an OIDC subject from an identity provider's "sub"
// claim.
func NewOIDCSubject(raw string) (OIDCSubject, error) {
	if !utf8.ValidString(raw) {
		return OIDCSubject{}, fmt.Errorf("%w: invalid UTF-8", ErrInvalidOIDCSubject)
	}
	if strings.ContainsRune(raw, 0) {
		return OIDCSubject{}, fmt.Errorf("%w: NUL bytes are not allowed", ErrInvalidOIDCSubject)
	}
	if count := utf8.RuneCountInString(raw); count < 1 || count > MaxOIDCSubjectLength {
		return OIDCSubject{}, ErrInvalidOIDCSubject
	}
	return OIDCSubject{value: raw}, nil
}

func (s OIDCSubject) String() string {
	return s.value
}

// NewUsername constructs a username from an identity provider's "username"
// claim.
func NewUsername(raw string) (Username, error) {
	if !utf8.ValidString(raw) {
		return Username{}, fmt.Errorf("%w: invalid UTF-8", ErrInvalidUsername)
	}
	if strings.ContainsRune(raw, 0) {
		return Username{}, fmt.Errorf("%w: NUL bytes are not allowed", ErrInvalidUsername)
	}
	if count := utf8.RuneCountInString(raw); count < 1 || count > MaxUsernameLength {
		return Username{}, ErrInvalidUsername
	}
	return Username{value: raw}, nil
}

func (u Username) String() string {
	return u.value
}

// NoEmailAddress is the sentinel value for a user with no known email
// address.
var NoEmailAddress = EmailAddress{}

// NewEmailAddress constructs an email address from an identity provider's
// "email" claim. It performs only a cursory sanity check, as PicoShare never
// sends mail to this address.
func NewEmailAddress(raw string) (EmailAddress, error) {
	if !utf8.ValidString(raw) {
		return EmailAddress{}, fmt.Errorf("%w: invalid UTF-8", ErrInvalidEmailAddress)
	}
	if !strings.Contains(raw, "@") || len(raw) > MaxEmailAddressBytes {
		return EmailAddress{}, ErrInvalidEmailAddress
	}
	return EmailAddress{value: raw}, nil
}

// Empty reports whether the identity provider supplied an email address.
func (e EmailAddress) Empty() bool {
	return e.value == ""
}

func (e EmailAddress) String() string {
	return e.value
}

// CanManageEntry reports whether u may view, edit, or delete entry.
func (u User) CanManageEntry(entry UploadMetadata) bool {
	return u.IsAdmin || (!entry.OwnerID.Empty() && entry.OwnerID == u.ID)
}

// OwnsGuestLink reports whether u created gl.
func (u User) OwnsGuestLink(gl GuestLink) bool {
	return !gl.OwnerID.Empty() && gl.OwnerID == u.ID
}
