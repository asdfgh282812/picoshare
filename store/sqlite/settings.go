package sqlite

import (
	"database/sql"
	"log"

	"github.com/mtlynch/picoshare/picoshare"
)

// We only store one set of settings at a time, so we used a fixed row ID.
const settingsRowID = 1

func (s Store) ReadSettings() (picoshare.Settings, error) {
	var expirationInDays uint16
	var retentionDays sql.NullInt16
	var defaultLanguageRaw sql.NullString
	if err := s.db.QueryRow(`
   SELECT
   	default_expiration_in_days,
   	download_history_retention_days,
   	default_language
   FROM
   	settings
   WHERE
   	id = :row_id`, sql.Named("row_id", settingsRowID)).Scan(&expirationInDays, &retentionDays, &defaultLanguageRaw); err != nil {
		if err == sql.ErrNoRows {
			return picoshare.Settings{}, nil
		}
		return picoshare.Settings{}, err
	}

	retention := picoshare.KeepDownloadHistoryForever
	if retentionDays.Valid {
		var err error
		if retention, err = picoshare.NewDownloadHistoryRetentionInDays(uint16(retentionDays.Int16)); err != nil {
			return picoshare.Settings{}, err
		}
	}

	defaultLanguage := picoshare.SiteDefaultLanguageAuto
	if defaultLanguageRaw.Valid {
		var err error
		if defaultLanguage, err = picoshare.NewSiteDefaultLanguage(defaultLanguageRaw.String); err != nil {
			return picoshare.Settings{}, err
		}
	}

	return picoshare.Settings{
		DefaultFileLifetime:      picoshare.NewFileLifetimeInDays(expirationInDays),
		DownloadHistoryRetention: retention,
		DefaultLanguage:          defaultLanguage,
	}, nil
}

func (s Store) UpdateSettings(settings picoshare.Settings) error {
	log.Printf("saving new settings: %s", settings)
	expirationInDays := settings.DefaultFileLifetime.Days()
	var retentionDays sql.NullInt16
	if !settings.DownloadHistoryRetention.IsForever() {
		retentionDays = sql.NullInt16{Int16: int16(settings.DownloadHistoryRetention.Days()), Valid: true}
	}
	var defaultLanguage sql.NullString
	if !settings.DefaultLanguage.IsAuto() {
		defaultLanguage = sql.NullString{String: settings.DefaultLanguage.String(), Valid: true}
	}
	if _, err := s.db.Exec(`
   UPDATE
   	settings
   SET
   	default_expiration_in_days = :expiration,
   	download_history_retention_days = :retention_days,
   	default_language = :default_language
   WHERE
   	id = :row_id`,
		sql.Named("expiration", expirationInDays),
		sql.Named("retention_days", retentionDays),
		sql.Named("default_language", defaultLanguage),
		sql.Named("row_id", settingsRowID)); err != nil {
		return err
	}

	return nil
}
