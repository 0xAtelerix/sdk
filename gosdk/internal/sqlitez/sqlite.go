package sqlitez

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	zsqlite "zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// OpenOptions captures SDK SQLite connection PRAGMAs owned by runtime readers.
type OpenOptions struct {
	QueryOnly                bool
	DisableWALAutoCheckpoint bool
}

// OpenConn opens a read-only SQLite connection with the SDK retry policy.
func OpenConn(
	ctx context.Context,
	dbPath string,
	opts OpenOptions,
) (*zsqlite.Conn, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&uri=true", dbPath)
	log.Info().Str("path", dsn).Msg("connecting to sqlite")

	maxTries := 50

	for {
		conn, err := zsqlite.OpenConn(dsn, zsqlite.OpenReadOnly|zsqlite.OpenURI)
		if err != nil {
			log.Error().Err(err).Msg("failed to open sqlite db")

			if retryErr := waitSQLiteRetry(ctx, &maxTries, err); retryErr != nil {
				return nil, retryErr
			}

			continue
		}

		if err := applyOpenOptions(conn, opts); err != nil {
			if closeErr := conn.Close(); closeErr != nil {
				log.Ctx(ctx).Warn().Err(closeErr).Msg("close sqlite db after pragma failure")
			}

			if retryErr := waitSQLiteRetry(ctx, &maxTries, err); retryErr != nil {
				return nil, retryErr
			}

			continue
		}

		log.Info().Str("path", dbPath).Msg("sqlite db opened")

		return conn, nil
	}
}

func applyOpenOptions(conn *zsqlite.Conn, opts OpenOptions) error {
	var script string
	if opts.QueryOnly {
		script += "PRAGMA query_only = ON;\n"
	}

	if opts.DisableWALAutoCheckpoint {
		script += "PRAGMA wal_autocheckpoint = 0;\n"
	}

	if script == "" {
		return nil
	}

	return sqlitex.ExecScript(conn, script)
}

func waitSQLiteRetry(ctx context.Context, maxTries *int, err error) error {
	if *maxTries == 0 {
		return err
	}

	*maxTries--

	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
