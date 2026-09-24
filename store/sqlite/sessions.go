package sqlite

import (
	"database/sql"
	"log"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
)

func (s Store) InsertSession(session picoshare.Session) error {
	if _, err := s.db.Exec(`
	INSERT INTO sessions
		(token_hash, user_id, creation_time, expiration_time)
	VALUES
		(:token_hash, :user_id, :creation_time, :expiration_time)`,
		sql.Named("token_hash", session.TokenHash.String()),
		sql.Named("user_id", session.UserID.Int64()),
		sql.Named("creation_time", formatTime(session.Created)),
		sql.Named("expiration_time", formatTime(session.Expires))); err != nil {
		return err
	}
	return nil
}

// GetSessionUser returns the user associated with an unexpired session. It
// joins against the users table on every call, rather than caching anything
// about the user in the session, so that a change to a user's admin status
// takes effect on their very next request.
func (s Store) GetSessionUser(hash picoshare.SessionTokenHash) (picoshare.User, error) {
	row := s.db.QueryRow(`
	SELECT
		users.id, users.oidc_subject, users.username, users.email, users.is_admin,
		users.creation_time, users.last_login_time
	FROM
		sessions
	INNER JOIN
		users ON sessions.user_id = users.id
	WHERE
		sessions.token_hash = :token_hash
		AND datetime(sessions.expiration_time) > datetime(:now)`,
		sql.Named("token_hash", hash.String()),
		sql.Named("now", formatTime(s.now())))

	user, err := userFromRow(row)
	if err == sql.ErrNoRows {
		return picoshare.User{}, store.SessionNotFoundError{}
	} else if err != nil {
		return picoshare.User{}, err
	}
	return user, nil
}

func (s Store) DeleteSession(hash picoshare.SessionTokenHash) error {
	if _, err := s.db.Exec(`
	DELETE FROM
		sessions
	WHERE
		token_hash = :token_hash`, sql.Named("token_hash", hash.String())); err != nil {
		return err
	}
	return nil
}

// deleteExpiredSessions removes sessions whose expiration_time has already
// passed. Purge calls this as part of routine database maintenance.
func (s Store) deleteExpiredSessions() error {
	log.Printf("deleting expired sessions from database")
	if _, err := s.db.Exec(`
	DELETE FROM
		sessions
	WHERE
		datetime(expiration_time) <= datetime(:now)`, sql.Named("now", formatTime(s.now()))); err != nil {
		return err
	}
	return nil
}
