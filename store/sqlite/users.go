package sqlite

import (
	"context"
	"database/sql"
	"log"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
)

// RecordUserLogin finds or creates the user identified by identity, updates
// their profile and last-login time, and returns the resulting user. The
// first user PicoShare ever sees becomes an administrator and claims every
// entry and guest link that predates multi-user support.
func (s Store) RecordUserLogin(identity picoshare.UserIdentity) (picoshare.User, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return picoshare.User{}, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("failed to rollback user login: %v", err)
		}
	}()

	now := formatTime(s.now())
	email := emailToNullString(identity.Email)

	var id int64
	err = tx.QueryRow(`
	SELECT
		id
	FROM
		users
	WHERE
		oidc_subject = :subject`, sql.Named("subject", identity.Subject.String())).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		isAdmin, insertErr := isFirstUser(tx)
		if insertErr != nil {
			return picoshare.User{}, insertErr
		}
		res, insertErr := tx.Exec(`
		INSERT INTO users
			(oidc_subject, username, email, is_admin, creation_time, last_login_time)
		VALUES
			(:subject, :username, :email, :is_admin, :now, :now)`,
			sql.Named("subject", identity.Subject.String()),
			sql.Named("username", identity.Username.String()),
			sql.Named("email", email),
			sql.Named("is_admin", isAdmin),
			sql.Named("now", now))
		if insertErr != nil {
			return picoshare.User{}, insertErr
		}
		id, insertErr = res.LastInsertId()
		if insertErr != nil {
			return picoshare.User{}, insertErr
		}
		if isAdmin {
			if claimErr := claimOwnerlessRows(tx, id); claimErr != nil {
				return picoshare.User{}, claimErr
			}
		}
	case err != nil:
		return picoshare.User{}, err
	default:
		if _, updateErr := tx.Exec(`
		UPDATE users
		SET
			username = :username,
			email = :email,
			last_login_time = :now
		WHERE
			id = :id`,
			sql.Named("username", identity.Username.String()),
			sql.Named("email", email),
			sql.Named("now", now),
			sql.Named("id", id)); updateErr != nil {
			return picoshare.User{}, updateErr
		}
	}

	user, err := userFromRow(tx.QueryRow(`
	SELECT
		id, oidc_subject, username, email, is_admin, preferred_language, creation_time, last_login_time
	FROM
		users
	WHERE
		id = :id`, sql.Named("id", id)))
	if err != nil {
		return picoshare.User{}, err
	}

	return user, tx.Commit()
}

// isFirstUser reports whether the users table is currently empty. It runs
// inside RecordUserLogin's transaction, so two logins racing to become the
// first user can't both succeed: SQLite's default transaction isolation
// aborts the loser with SQLITE_BUSY_SNAPSHOT, and that login simply retries.
func isFirstUser(tx *sql.Tx) (bool, error) {
	var count int
	if err := tx.QueryRow(`SELECT count(*) FROM users`).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

// claimOwnerlessRows assigns every entry and guest link that predates
// multi-user support to the newly created administrator.
func claimOwnerlessRows(tx *sql.Tx, adminID int64) error {
	if _, err := tx.Exec(`
	UPDATE entries
	SET owner_user_id = :admin_id
	WHERE owner_user_id IS NULL`, sql.Named("admin_id", adminID)); err != nil {
		return err
	}
	if _, err := tx.Exec(`
	UPDATE guest_links
	SET owner_user_id = :admin_id
	WHERE owner_user_id IS NULL`, sql.Named("admin_id", adminID)); err != nil {
		return err
	}
	return nil
}

func (s Store) GetUsers() ([]picoshare.User, error) {
	rows, err := s.db.Query(`
	SELECT
		id, oidc_subject, username, email, is_admin, preferred_language, creation_time, last_login_time
	FROM
		users
	ORDER BY
		id`)
	if err != nil {
		return nil, err
	}

	users := []picoshare.User{}
	for rows.Next() {
		u, err := userFromRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s Store) GrantAdmin(id picoshare.UserID) error {
	res, err := s.db.Exec(`
	UPDATE users
	SET is_admin = 1
	WHERE id = :id`, sql.Named("id", id.Int64()))
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return store.UserNotFoundError{ID: id}
	}
	return nil
}

// RevokeAdmin demotes the given user, unless doing so would leave PicoShare
// with no administrators. The check and the update happen in a single
// statement so that two concurrent demotions can't both succeed and leave
// zero administrators.
func (s Store) RevokeAdmin(id picoshare.UserID) error {
	res, err := s.db.Exec(`
	UPDATE users
	SET is_admin = 0
	WHERE
		id = :id
		AND is_admin = 1
		AND (SELECT count(*) FROM users WHERE is_admin = 1) > 1`,
		sql.Named("id", id.Int64()))
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows > 0 {
		return nil
	}

	// No rows changed: figure out why, so we can report the right error.
	var isAdmin bool
	err = s.db.QueryRow(`SELECT is_admin FROM users WHERE id = :id`, sql.Named("id", id.Int64())).Scan(&isAdmin)
	if err == sql.ErrNoRows {
		return store.UserNotFoundError{ID: id}
	} else if err != nil {
		return err
	}
	if !isAdmin {
		// Already not an admin: treat as a successful no-op.
		return nil
	}
	return store.LastAdminError{}
}

// UpdateUserLanguage sets the user's preferred interface language.
func (s Store) UpdateUserLanguage(id picoshare.UserID, lang picoshare.Language) error {
	res, err := s.db.Exec(`
	UPDATE users
	SET preferred_language = :language
	WHERE id = :id`,
		sql.Named("language", lang.String()),
		sql.Named("id", id.Int64()))
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return store.UserNotFoundError{ID: id}
	}
	return nil
}

func userFromRow(row rowScanner) (picoshare.User, error) {
	var id int64
	var subjectRaw string
	var usernameRaw string
	var emailRaw *string
	var isAdmin bool
	var preferredLanguageRaw *string
	var creationTimeRaw string
	var lastLoginTimeRaw string

	if err := row.Scan(&id, &subjectRaw, &usernameRaw, &emailRaw, &isAdmin, &preferredLanguageRaw, &creationTimeRaw, &lastLoginTimeRaw); err != nil {
		return picoshare.User{}, err
	}

	subject, err := picoshare.NewOIDCSubject(subjectRaw)
	if err != nil {
		return picoshare.User{}, err
	}
	username, err := picoshare.NewUsername(usernameRaw)
	if err != nil {
		return picoshare.User{}, err
	}
	email := picoshare.NoEmailAddress
	if emailRaw != nil {
		if email, err = picoshare.NewEmailAddress(*emailRaw); err != nil {
			return picoshare.User{}, err
		}
	}
	var preferredLanguage picoshare.Language
	if preferredLanguageRaw != nil {
		if preferredLanguage, err = picoshare.NewLanguage(*preferredLanguageRaw); err != nil {
			return picoshare.User{}, err
		}
	}
	created, err := parseDatetime(creationTimeRaw)
	if err != nil {
		return picoshare.User{}, err
	}
	lastLogin, err := parseDatetime(lastLoginTimeRaw)
	if err != nil {
		return picoshare.User{}, err
	}

	return picoshare.User{
		ID:                picoshare.UserIDFromInt64(id),
		Subject:           subject,
		Username:          username,
		Email:             email,
		IsAdmin:           isAdmin,
		PreferredLanguage: preferredLanguage,
		Created:           created,
		LastLogin:         lastLogin,
	}, nil
}

func emailToNullString(e picoshare.EmailAddress) *string {
	if e.Empty() {
		return nil
	}
	v := e.String()
	return &v
}
