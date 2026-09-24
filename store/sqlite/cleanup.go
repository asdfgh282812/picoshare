package sqlite

import (
	"context"
	"database/sql"
	"log"
)

// Purge deletes expired entries and download history, clears orphaned rows
// from the database, and returns the freed space to the filesystem.
func (s Store) Purge() error {
	log.Printf("deleting expired entries and orphaned data from database")
	if err := s.deleteExpiredEntries(); err != nil {
		return err
	}

	if err := s.deleteOrphanedRows(); err != nil {
		return err
	}

	if err := s.deleteExpiredDownloadHistory(); err != nil {
		return err
	}

	if err := s.reclaimFreePages(); err != nil {
		return err
	}

	return nil
}

// ReclaimableBytes returns the number of bytes in the database file that
// PicoShare can return to the filesystem.
func (s Store) ReclaimableBytes() (uint64, error) {
	var freePages, pageSize uint64
	if err := s.db.QueryRow(`PRAGMA freelist_count`).Scan(&freePages); err != nil {
		return 0, err
	}
	if err := s.db.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		return 0, err
	}
	return freePages * pageSize, nil
}

func (s Store) deleteExpiredEntries() error {
	log.Printf("deleting expired entries from database")

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}

	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("failed to rollback delete expired entries: %v", err)
		}
	}()

	currentTime := formatTime(s.now())

	if _, err = tx.Exec(`
   DELETE FROM
   	downloads
   WHERE
   	entry_id IN (
   		SELECT
   			id
   		FROM
   			entries
   		WHERE
   			entries.expiration_time IS NOT NULL AND
   			entries.expiration_time < :current_time
   	);`, sql.Named("current_time", currentTime)); err != nil {
		return err
	}

	if _, err = tx.Exec(`
   DELETE FROM
   	entries_data
   WHERE
   	id IN (
   		SELECT
   			id
   		FROM
   			entries
   		WHERE
   			entries.expiration_time IS NOT NULL AND
   			entries.expiration_time < :current_time
   	);`, sql.Named("current_time", currentTime)); err != nil {
		return err
	}

	if _, err = tx.Exec(`
   DELETE FROM
   	entries
   WHERE
   	entries.expiration_time IS NOT NULL AND
   	entries.expiration_time < :current_time;
   `, sql.Named("current_time", currentTime)); err != nil {
		return err
	}

	return tx.Commit()
}

func (s Store) deleteOrphanedRows() error {
	log.Printf("purging orphaned rows from database")

	// Delete rows from entries_data if they don't reference valid rows in
	// entries. This can happen if the entry insertion fails partway through.
	rows, err := s.db.Exec(`
   	DELETE FROM
   		entries_data
   	WHERE
   	id IN (
   		SELECT
   			DISTINCT entries_data.id AS entry_id
   		FROM
   			entries_data
   		LEFT JOIN
   			entries ON entries_data.id = entries.id
   		WHERE
   			entries.id IS NULL
   		)`)
	if err != nil {
		return err
	}

	ra, err := rows.RowsAffected()
	if err != nil {
		return err
	}

	log.Printf("purge completed successfully (%d rows affected)", ra)

	return nil
}

func (s Store) deleteExpiredDownloadHistory() error {
	settings, err := s.ReadSettings()
	if err != nil {
		return err
	}
	if settings.DownloadHistoryRetention.IsForever() {
		return nil
	}

	log.Printf("deleting download history older than %s", settings.DownloadHistoryRetention)
	cutoff := settings.DownloadHistoryRetention.CutoffFrom(s.now())
	if _, err := s.db.Exec(`
	DELETE FROM
		downloads
	WHERE
		datetime(download_timestamp) < datetime(:cutoff)`, sql.Named("cutoff", formatTime(cutoff))); err != nil {
		return err
	}

	return nil
}

func (s Store) reclaimFreePages() error {
	log.Printf("returning free database pages to the filesystem")

	// SQLite frees one page per step of incremental_vacuum, so read the
	// statement to completion rather than using Exec, which may stop early.
	rows, err := s.db.Query(`PRAGMA incremental_vacuum`)
	if err != nil {
		return err
	}
	for rows.Next() {
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	// Litestream manages WAL checkpoints itself, so only shrink the WAL file
	// when Litestream isn't running.
	if !s.optimizeForLitestream {
		if _, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
			return err
		}
	}

	return nil
}
