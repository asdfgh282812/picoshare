package store

import (
	"fmt"

	"github.com/mtlynch/picoshare/picoshare"
)

// EntryNotFoundError occurs when no entry exists with the given ID.
type EntryNotFoundError struct {
	ID picoshare.EntryID
}

func (f EntryNotFoundError) Error() string {
	return fmt.Sprintf("Could not find entry with ID %v", f.ID)
}

// GuestLinkNotFoundError occurs when no guest link exists with the given ID.
type GuestLinkNotFoundError struct {
	ID picoshare.GuestLinkID
}

func (f GuestLinkNotFoundError) Error() string {
	return fmt.Sprintf("Could not find guest link with ID %v", f.ID)
}

// UserNotFoundError occurs when no user exists with the given ID.
type UserNotFoundError struct {
	ID picoshare.UserID
}

func (f UserNotFoundError) Error() string {
	return fmt.Sprintf("Could not find user with ID %v", f.ID)
}

// SessionNotFoundError occurs when no active session matches a given token.
type SessionNotFoundError struct{}

func (f SessionNotFoundError) Error() string {
	return "Could not find an active session for the given token"
}

// LastAdminError occurs when an operation would leave PicoShare with no
// administrators.
type LastAdminError struct{}

func (f LastAdminError) Error() string {
	return "Cannot remove the last administrator"
}
