package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domain "github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/common"
)

// timeLayout is how created_at is stored: RFC 3339 in UTC, with a fixed
// nine-digit fraction.
//
// [time.RFC3339Nano] trims trailing zeros from the fraction. That round-trips
// the instant exactly, but it makes the stored text sort wrong against its
// neighbours — "…:05.1Z" lands after "…:05.05Z" — and a TEXT column's natural
// order is the only order SQLite has for it. Nothing sorts on created_at yet;
// nine characters a row now is cheaper than a data migration when something
// does.
const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// CharacterRepository is a [domain.CharacterRepository] backed by SQLite.
//
// It holds the [DB] rather than a [database/sql.DB] so that the pool's
// lifecycle stays with whoever owns startup: a repository built before Open, or
// used after Close, reports [ErrClosed] instead of panicking on a nil handle.
type CharacterRepository struct {
	db *DB
}

// NewCharacterRepository returns a repository reading and writing db.
//
// db need not be open yet — the composition root builds its services before it
// starts anything — but it must not be nil, which would only ever be a wiring
// mistake and is worth failing loudly at startup rather than at the first
// request.
func NewCharacterRepository(db *DB) *CharacterRepository {
	if db == nil {
		panic("sqlite: db must be set")
	}
	return &CharacterRepository{db: db}
}

var _ domain.CharacterRepository = (*CharacterRepository)(nil)

// characterFactory is the domain's door for adapters: Save reaches through it
// to advance an aggregate's version, which is not something callers may do, and
// Find to rebuild a Character out of stored columns.
var characterFactory domain.CharacterFactory

// saveCharacter is an upsert whose ON CONFLICT clause carries the compare-and-
// set, so the whole of [CharacterRepository.Save] is one statement in SQLite's
// implicit transaction. See Save for what each outcome means.
//
// created_at is absent from the DO UPDATE on purpose: not assigning it is
// what preserves it across an update.
const saveCharacter = `
INSERT INTO characters (id, data, created_at, version)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	data    = excluded.data,
	version = characters.version + 1
WHERE characters.version = ?
RETURNING version`

// Save implements [domain.CharacterRepository].
func (r *CharacterRepository) Save(ctx context.Context, c *domain.Character) error {
	pool, err := r.db.handle()
	if err != nil {
		return err
	}

	var stored uint64
	err = retry(ctx, func() error {
		return pool.QueryRowContext(ctx, saveCharacter,
			c.ID().String(),
			// A string, not the [encoding/json.RawMessage] itself. database/sql
			// binds a []byte as a BLOB, and SQLite's TEXT affinity does not
			// convert BLOBs, so the sheet would land as a blob — where
			// json_valid switches to JSONB rules and the STRICT table refuses
			// it outright. Converting also copies, so the stored value cannot
			// share a backing array with the caller's.
			string(c.Data()),
			c.CreatedAt().UTC().Format(timeLayout),
			c.Version().Uint64(),
			c.Version().Uint64(),
		).Scan(&stored)
	})

	switch {
	// The DO UPDATE's WHERE was false: the stored Character has moved past the
	// revision c was loaded at. SQLite does not treat that as an error — it
	// leaves the row alone, changes nothing, and returns no row — so the missing
	// row is the failed compare-and-set, and this is where a lost update is
	// reported instead of silently accepted.
	case errors.Is(err, sql.ErrNoRows):
		return common.ErrConflict
	case err != nil:
		return fmt.Errorf("sqlite: save character %s: %w", c.ID(), err)
	}

	// An insert stores the version the aggregate was constructed with and
	// returns it unchanged; a replacement returns one past it. Comparing is what
	// tells the two apart without a second round trip. Advancing c itself, not
	// just the stored row, lets the caller edit and save again without
	// reloading.
	if stored != c.Version().Uint64() {
		characterFactory.NextVersion(c)
	}

	return nil
}

const findCharacter = `SELECT data, created_at, version FROM characters WHERE id = ?`

// Find implements [domain.CharacterRepository].
func (r *CharacterRepository) Find(ctx context.Context, id domain.CharacterID) (domain.Character, error) {
	pool, err := r.db.handle()
	if err != nil {
		return domain.Character{}, err
	}

	var (
		// data is scanned as a string rather than a []byte so the conversion
		// below allocates: the sheet handed to Rehydrate cannot alias anything
		// the driver still owns. Never [database/sql.RawBytes], which is only
		// valid until the next row.
		data      string
		createdAt string
		version   int64
	)
	err = retry(ctx, func() error {
		return pool.QueryRowContext(ctx, findCharacter, id.String()).
			Scan(&data, &createdAt, &version)
	})

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return domain.Character{}, common.ErrNotFound
	case err != nil:
		return domain.Character{}, fmt.Errorf("sqlite: find character %s: %w", id, err)
	}

	// Parsed here rather than scanned into a [time.Time]: the column is declared
	// TEXT, so the driver hands it over untouched. Declaring it DATETIME would
	// hand the choice of Go type to the driver's own conversion instead.
	created, err := time.Parse(timeLayout, createdAt)
	if err != nil {
		return domain.Character{}, fmt.Errorf("sqlite: character %s: parse created_at: %w", id, err)
	}

	// RehydrateVersion rather than ParseVersion: a value read back out of our
	// own column is not a client-supplied one, and rejecting it here would only
	// turn stored data into a runtime error.
	return characterFactory.Rehydrate(
		id,
		json.RawMessage(data),
		created.UTC(),
		common.RehydrateVersion(uint64(version)),
	), nil
}

const listCharacter = `SELECT id, data, created_at, version FROM characters ORDER BY id ASC`

// List implements [character.CharacterRepository].
func (r *CharacterRepository) List(ctx context.Context) ([]domain.Character, error) {
	pool, err := r.db.handle()
	if err != nil {
		return nil, err
	}

	var entities []domain.Character

	if err := retry(ctx, func() error {
		rows, err := pool.QueryContext(ctx, listCharacter)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var (
				id string
				// data is scanned as a string rather than a []byte so the conversion
				// below allocates: the sheet handed to Rehydrate cannot alias anything
				// the driver still owns. Never [database/sql.RawBytes], which is only
				// valid until the next row.
				data      string
				createdAt string
				version   int64
			)
			if err := rows.Scan(&id, &data, &createdAt, &version); err != nil {
				return err
			}

			created, err := time.Parse(timeLayout, createdAt)
			if err != nil {
				return fmt.Errorf("sqlite: character: parse created_at: %w", err)
			}

			entities = append(entities,
				characterFactory.Rehydrate(domain.CharacterID(id), json.RawMessage(data), created, common.RehydrateVersion(uint64(version))))

		}
		if err := rows.Err(); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("sqlite: list character: %w", err)
	}

	return entities, nil
}
