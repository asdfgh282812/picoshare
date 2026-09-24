package handlers

import (
	"io"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
)

type Store interface {
	GetEntriesMetadata(opts ...store.ReadEntriesOption) ([]picoshare.UploadMetadata, error)
	ReadEntryFile(picoshare.EntryID) (io.ReadSeeker, error)
	GetEntryMetadata(id picoshare.EntryID) (picoshare.UploadMetadata, error)
	InsertEntry(reader io.Reader, metadata picoshare.UploadMetadata) error
	UpdateEntryMetadata(id picoshare.EntryID, metadata picoshare.UploadMetadata) error
	DeleteEntry(id picoshare.EntryID) error
	GetGuestLink(picoshare.GuestLinkID) (picoshare.GuestLink, error)
	GetGuestLinks(owner picoshare.UserID) ([]picoshare.GuestLink, error)
	InsertGuestLink(picoshare.GuestLink) error
	DeleteGuestLink(picoshare.GuestLinkID) error
	DisableGuestLink(picoshare.GuestLinkID) error
	EnableGuestLink(picoshare.GuestLinkID) error
	InsertEntryDownload(picoshare.EntryID, picoshare.DownloadRecord) error
	GetEntryDownloads(id picoshare.EntryID) ([]picoshare.DownloadRecord, error)
	ReadSettings() (picoshare.Settings, error)
	UpdateSettings(picoshare.Settings) error
	ReclaimableBytes() (uint64, error)

	// Users and sessions.
	RecordUserLogin(picoshare.UserIdentity) (picoshare.User, error)
	GetUsers() ([]picoshare.User, error)
	GrantAdmin(picoshare.UserID) error
	RevokeAdmin(picoshare.UserID) error
	InsertSession(picoshare.Session) error
	GetSessionUser(picoshare.SessionTokenHash) (picoshare.User, error)
	DeleteSession(picoshare.SessionTokenHash) error
	UpdateUserLanguage(picoshare.UserID, picoshare.Language) error

	// OIDC settings.
	ReadOIDCSettings() (picoshare.OIDCSettings, error)
	UpdateOIDCSettings(picoshare.OIDCSettings) error
	ClearOIDCSettings() error
	NeedsSetup() (bool, error)
	GenerateSetupToken() (picoshare.SetupToken, error)
	ValidateSetupToken(picoshare.SetupToken) (bool, error)
}
