package sqlite

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/mtlynch/picoshare/picoshare"
)

const (
	timeFormat = time.RFC3339
	// I think Chrome reads in 32768 chunks, but I haven't checked rigorously.
	defaultChunkSize = uint64(32768 * 10)
)

type (
	Params struct {
		Path                  string
		OptimizeForLitestream bool
		Now                   func() time.Time
		PicoShareChunkSize    uint64
	}

	Store struct {
		db                    *sql.DB
		chunkSize             uint64
		now                   func() time.Time
		optimizeForLitestream bool
	}

	rowScanner interface {
		Scan(...any) error
	}
)

func New(params Params) Store {
	log.Printf("reading DB from %s", params.Path)
	db, err := sql.Open("sqlite3", params.Path)
	if err != nil {
		log.Fatalln(err)
	}

	if _, err := db.Exec(`
		PRAGMA temp_store = FILE;
		PRAGMA journal_mode = WAL;
		PRAGMA foreign_keys = 1;
		`); err != nil {
		log.Fatalf("failed to set pragmas: %v", err)
	}

	if params.OptimizeForLitestream {
		if _, err := db.Exec(`
			-- Apply Litestream recommendations: https://litestream.io/tips/
			PRAGMA busy_timeout = 5000;
			PRAGMA synchronous = NORMAL;
			PRAGMA wal_autocheckpoint = 0;
				`); err != nil {
			log.Fatalf("failed to set Litestream compatibility pragmas: %v", err)
		}
	}

	enableIncrementalVacuum(db)

	applyMigrations(db)

	chunkSize := params.PicoShareChunkSize
	if chunkSize == 0 {
		chunkSize = defaultChunkSize
	}

	return Store{
		db:                    db,
		chunkSize:             chunkSize,
		now:                   params.Now,
		optimizeForLitestream: params.OptimizeForLitestream,
	}
}

// enableIncrementalVacuum lets Purge return the space of deleted files to the
// filesystem. Switching an existing database to incremental auto-vacuum
// requires a one-time VACUUM, which temporarily needs free disk space roughly
// equal to the size of the database.
func enableIncrementalVacuum(db *sql.DB) {
	const autoVacuumIncremental = 2
	var mode int
	if err := db.QueryRow(`PRAGMA auto_vacuum`).Scan(&mode); err != nil {
		log.Fatalf("failed to read auto_vacuum mode: %v", err)
	}
	if mode == autoVacuumIncremental {
		return
	}

	log.Printf("enabling incremental auto-vacuum, which may take a while on large databases")
	if _, err := db.Exec(`
		PRAGMA auto_vacuum = INCREMENTAL;
		VACUUM;
		`); err != nil {
		// Don't prevent startup, as the most likely cause is a lack of disk space
		// for VACUUM. PicoShare still reuses the space of deleted files for future
		// uploads, but it can't shrink the database file.
		log.Printf("failed to enable incremental auto-vacuum: %v", err)
	}
}

func formatExpirationTime(et picoshare.ExpirationTime) string {
	return formatTime(time.Time(et))
}

func formatTime(t time.Time) string {
	return t.UTC().Format(timeFormat)
}

func formatFileLifetime(lt picoshare.FileLifetime) string {
	return lt.String()
}

func parseDatetime(s string) (time.Time, error) {
	return time.Parse(timeFormat, s)
}

func parseFileLifetime(s string) (picoshare.FileLifetime, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return picoshare.FileLifetime{}, err
	}
	return picoshare.NewFileLifetimeFromDuration(d)
}
