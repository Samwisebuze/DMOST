package sqlite

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	// Aliased because its package name is "sqlite", the same as ours. This is
	// also the import that registers the driver as [DriverName], from that
	// package's init function.
	sqlitedriver "modernc.org/sqlite"
)

const (
	// maxRetries is how many times a blocked statement is re-issued before its
	// error is returned. Five attempts at up to 8ms apart bounds the added
	// latency at roughly 40ms, which is well inside a request and far short of
	// BusyTimeout.
	maxRetries = 5

	// minBackoff and maxBackoff bound the wait between attempts. The range is
	// what keeps two blocked callers from re-colliding in lockstep; the floor is
	// nonzero so a retry yields the connection rather than spinning on it.
	minBackoff = 1 * time.Millisecond
	maxBackoff = 8 * time.Millisecond
)

// SQLite primary result codes. The driver reports extended codes — 262 for
// SQLITE_LOCKED_SHAREDCACHE, 517 for SQLITE_BUSY_SNAPSHOT — whose low eight bits
// are the primary code, which is the granularity worth reacting to here.
const (
	sqliteBusy   = 5
	sqliteLocked = 6
)

// retry runs fn, re-issuing it while SQLite reports the statement blocked.
//
// In practice this is SQLITE_BUSY on a file-backed database: two writers
// overlapped and the loser waited out [DB.BusyTimeout] without the lock coming
// free. Giving it a few more short attempts turns an error the caller cannot do
// anything useful with into a slightly slower success.
//
// SQLITE_LOCKED is covered too, but rarely arrives. The shared-cache table-lock
// conflict an in-memory database could produce is handled inside the driver,
// which waits on sqlite3_unlock_notify rather than returning anything — and
// [DB.Open] keeps such a pool to one working connection so it does not come up
// at all. It is matched here because a code that does reach a caller means the
// same thing as SQLITE_BUSY: nothing is wrong with the statement, something
// else was holding the database.
//
// It cannot mask a lost update. A failed compare-and-set arrives as
// [database/sql.ErrNoRows], which is not a driver error and not a lock, so it
// returns from the first attempt exactly as a success would.
//
// The error from the last attempt is what comes back when the attempts run out,
// so the log says what SQLite said.
func retry(ctx context.Context, fn func() error) error {
	var err error
	for attempt := range maxRetries {
		if err = fn(); !isLocked(err) {
			return err
		}

		// No point sleeping after the final attempt.
		if attempt == maxRetries-1 {
			break
		}

		if err := sleep(ctx, backoff()); err != nil {
			return err
		}
	}
	return err
}

// isLocked reports whether err is SQLite refusing to proceed because something
// else holds a lock, as opposed to refusing the statement itself.
func isLocked(err error) bool {
	var serr *sqlitedriver.Error
	if !errors.As(err, &serr) {
		return false
	}

	// An extended code is its primary code with a subcode shifted into the high
	// bits — SQLITE_LOCKED_SHAREDCACHE is 6|1<<8, or 262 — so masking to the low
	// byte recovers the primary and matches every member of a family at once,
	// including ones SQLite has yet to define.
	switch serr.Code() & 0xff {
	case sqliteBusy, sqliteLocked:
		return true
	default:
		return false
	}
}

// backoff returns a wait in [minBackoff, maxBackoff]. math/rand/v2's global
// source needs no seeding and is safe to call from several goroutines, which
// matters because every concurrent request lands here.
func backoff() time.Duration {
	return minBackoff + rand.N(maxBackoff-minBackoff+1)
}

// sleep waits for d, or reports why it stopped waiting early. A cancelled
// request should not go on retrying a statement nobody is waiting for.
func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
