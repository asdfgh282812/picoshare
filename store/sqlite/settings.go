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
	if err := s.db.QueryRow(`
   SELECT
   	default_expiration_in_days,
   	download_history_retention_days
   FROM
   	settings
   WHERE
   	id = :row_id`, sql.Named("row_id", settingsRowID)).Scan(&expirationInDays, &retentionDays); err != nil {
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

	return picoshare.Settings{
		DefaultFileLifetime:      picoshare.NewFileLifetimeInDays(expirationInDays),
		DownloadHistoryRetention: retention,
	}, nil
}

func (s Store) UpdateSettings(settings picoshare.Settings) error {
	log.Printf("saving new settings: %s", settings)
	expirationInDays := settings.DefaultFileLifetime.Days()
	var retentionDays sql.NullInt16
	if !settings.DownloadHistoryRetention.IsForever() {
		retentionDays = sql.NullInt16{Int16: int16(settings.DownloadHistoryRetention.Days()), Valid: true}
	}
	if _, err := s.db.Exec(`
   UPDATE
   	settings
   SET
   	default_expiration_in_days = :expiration,
   	download_history_retention_days = :retention_days
   WHERE
   	id = :row_id`,
		sql.Named("expiration", expirationInDays),
		sql.Named("retention_days", retentionDays),
		sql.Named("row_id", settingsRowID)); err != nil {
		return err
	}

	return nil
}
