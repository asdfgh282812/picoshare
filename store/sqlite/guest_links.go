package sqlite

import (
	"context"
	"database/sql"
	"log"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
)

const guestLinkSelectColumns = `
			guest_links.id AS id,
			guest_links.label AS label,
			guest_links.is_disabled As is_disabled,
			guest_links.max_file_bytes AS max_file_bytes,
			guest_links.max_file_uploads AS max_file_uploads,
			guest_links.creation_time AS creation_time,
			guest_links.url_expiration_time AS url_expiration_time,
			guest_links.file_expiration_time AS file_expiration_time,
			guest_links.owner_user_id AS owner_user_id,
			SUM(CASE WHEN entries.id IS NOT NULL THEN 1 ELSE 0 END) AS entry_count`

func (s Store) GetGuestLink(id picoshare.GuestLinkID) (picoshare.GuestLink, error) {
	row := s.db.QueryRow(`
		SELECT`+guestLinkSelectColumns+`
		FROM
			guest_links
		LEFT JOIN
			entries ON guest_links.id = entries.guest_link_id
		WHERE
			guest_links.id=:id
		GROUP BY
			guest_links.id`, sql.Named("id", id))

	return guestLinkFromRow(row)
}

// GetGuestLinks returns every guest link owned by the given user. Unlike
// entries, PicoShare has no admin view across every user's guest links, so
// callers always filter by owner.
func (s Store) GetGuestLinks(owner picoshare.UserID) ([]picoshare.GuestLink, error) {
	rows, err := s.db.Query(`
		SELECT`+guestLinkSelectColumns+`
		FROM
			guest_links
		LEFT JOIN
			entries ON guest_links.id = entries.guest_link_id
		WHERE
			guest_links.owner_user_id = :owner_user_id
		GROUP BY
			guest_links.id`, sql.Named("owner_user_id", owner.Int64()))
	if err != nil {
		return []picoshare.GuestLink{}, err
	}

	gls := []picoshare.GuestLink{}
	for rows.Next() {
		gl, err := guestLinkFromRow(rows)
		if err != nil {
			return []picoshare.GuestLink{}, err
		}

		gls = append(gls, gl)
	}

	return gls, nil
}

func (s *Store) InsertGuestLink(guestLink picoshare.GuestLink) error {
	log.Printf("saving new guest link %s", guestLink.ID)

	var ownerUserID *int64
	if !guestLink.OwnerID.Empty() {
		v := guestLink.OwnerID.Int64()
		ownerUserID = &v
	}

	if _, err := s.db.Exec(`
	INSERT INTO guest_links
		(
			id,
			label,
			is_disabled,
			max_file_bytes,
			max_file_uploads,
			creation_time,
			url_expiration_time,
			file_expiration_time,
			owner_user_id
		)
		VALUES (:id, :label, :is_disabled,:max_file_bytes, :max_file_uploads, :creation_time, :url_expiration_time, :file_expiration_time, :owner_user_id)
	`,
		sql.Named("id", guestLink.ID),
		sql.Named("label", guestLink.Label),
		sql.Named("is_disabled", guestLink.IsDisabled),
		sql.Named("max_file_bytes", guestLink.MaxFileBytes),
		sql.Named("max_file_uploads", guestLink.MaxFileUploads),
		sql.Named("creation_time", formatTime(guestLink.Created)),
		sql.Named("url_expiration_time", formatExpirationTime(guestLink.UrlExpires)),
		sql.Named("file_expiration_time", formatFileLifetime(guestLink.MaxFileLifetime)),
		sql.Named("owner_user_id", ownerUserID)); err != nil {
		return err
	}

	return nil
}

func (s Store) DeleteGuestLink(id picoshare.GuestLinkID) error {
	log.Printf("deleting guest link %s", id)

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}

	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("failed to rollback delete guest link: %v", err)
		}
	}()

	if _, err = tx.Exec(`
	UPDATE
		entries
	SET
		guest_link_id = NULL
	WHERE
		guest_link_id = :id`, sql.Named("id", id)); err != nil {
		log.Printf("removing references to guest link %s from entries table failed: %v", id, err)
		return err
	}

	if _, err = tx.Exec(`
	DELETE FROM
		guest_links
	WHERE
		id=:id`, sql.Named("id", id)); err != nil {
		log.Printf("deleting %s from guest_links table failed: %v", id, err)
		return err
	}

	return tx.Commit()
}

func (s Store) DisableGuestLink(id picoshare.GuestLinkID) error {
	log.Printf("disabling guest link %s", id)

	_, err := s.db.Exec(`
    UPDATE
        guest_links
    SET
        is_disabled = 1
    WHERE
        id = :id`, sql.Named("id", id))

	if err != nil {
		log.Printf("disabling guest link %s failed: %v", id, err)
		return err
	}

	return nil
}

func (s Store) EnableGuestLink(id picoshare.GuestLinkID) error {
	log.Printf("enabling guest link %s", id)

	_, err := s.db.Exec(`
	UPDATE
		guest_links
	SET
		is_disabled = 0
	WHERE
		id = :id`, sql.Named("id", id))

	if err != nil {
		log.Printf("enabling guest link %s failed: %v", id, err)
		return err
	}

	return nil
}

func guestLinkFromRow(row rowScanner) (picoshare.GuestLink, error) {
	var id picoshare.GuestLinkID
	var label picoshare.GuestLinkLabel
	var isDisabled bool
	var maxFileBytes picoshare.GuestUploadMaxFileBytes
	var maxFileUploads picoshare.GuestUploadCountLimit
	var creationTimeRaw string
	var urlExpirationTimeRaw string
	var fileLifetimeRaw *string
	var ownerUserID *int64
	var filesUploaded int

	err := row.Scan(&id, &label, &isDisabled, &maxFileBytes, &maxFileUploads, &creationTimeRaw, &urlExpirationTimeRaw, &fileLifetimeRaw, &ownerUserID, &filesUploaded)
	if err == sql.ErrNoRows {
		return picoshare.GuestLink{}, store.GuestLinkNotFoundError{ID: id}
	} else if err != nil {
		return picoshare.GuestLink{}, err
	}

	ct, err := parseDatetime(creationTimeRaw)
	if err != nil {
		return picoshare.GuestLink{}, err
	}

	uet, err := parseDatetime(urlExpirationTimeRaw)
	if err != nil {
		return picoshare.GuestLink{}, err
	}

	var fileLifetime picoshare.FileLifetime
	if fileLifetimeRaw == nil {
		fileLifetime = picoshare.FileLifetimeInfinite
	} else {
		fileLifetime, err = parseFileLifetime(*fileLifetimeRaw)
		if err != nil {
			return picoshare.GuestLink{}, err
		}
	}

	var ownerID picoshare.UserID
	if ownerUserID != nil {
		ownerID = picoshare.UserIDFromInt64(*ownerUserID)
	}

	return picoshare.GuestLink{
		ID:              id,
		Label:           label,
		OwnerID:         ownerID,
		IsDisabled:      isDisabled,
		MaxFileBytes:    maxFileBytes,
		MaxFileUploads:  maxFileUploads,
		FilesUploaded:   filesUploaded,
		Created:         ct,
		UrlExpires:      picoshare.ExpirationTime(uet),
		MaxFileLifetime: fileLifetime,
	}, nil
}
