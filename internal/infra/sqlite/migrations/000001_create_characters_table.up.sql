-- The character aggregate: pkg/domain/character.Character is an identity, an
-- opaque sheet, a creation instant, and a version, and this table is those four
-- and nothing else.
--
-- STRICT is load-bearing. Without it SQLite stores whatever it is handed
-- regardless of the declared type, and database/sql sends a Go []byte as a BLOB
-- which TEXT affinity does not convert -- so a Save binding c.Data() instead of
-- string(c.Data()) would store a blob silently and forever. With it, the same
-- mistake is "cannot store BLOB value in TEXT column characters.data". It is the
-- same argument DB.ForeignKeys makes: a constraint the engine does not enforce
-- is worse than no constraint at all.
--
-- There is deliberately no IF NOT EXISTS. The schema_migrations table is what
-- makes this run once; IF NOT EXISTS would paper over the one failure mode
-- golang-migrate leaves detectable, where a lost version row replays this
-- migration against a database that already has the table.
CREATE TABLE characters (
    -- A UUIDv7 in its canonical string form, so the primary key is also
    -- time-ordered. No WITHOUT ROWID: the sheet is an arbitrarily large document
    -- and does not belong in the b-tree's interior pages.
    id         TEXT    NOT NULL PRIMARY KEY,

    -- TEXT rather than BLOB so json_valid applies JSON *text* rules. Since
    -- SQLite 3.45 a BLOB argument to a JSON function is read as JSONB, which is
    -- a different format with different answers.
    --
    -- The CHECK is a backstop, not the rule. The rule is
    -- character.validateSheet, which additionally requires a JSON object;
    -- reaching this constraint means a bug in internal/infra/sqlite rather than
    -- a bad request.
    data       TEXT    NOT NULL CHECK (json_valid(data)),

    -- RFC 3339 in UTC with a fixed nine-digit fraction, so the text sorts in the
    -- same order as the instants it encodes. See timeLayout in character.go.
    created_at TEXT    NOT NULL,

    -- common.Version, which starts at 1 and only ever advances. The compare-and-
    -- set in CharacterRepository.Save reads and writes this column.
    version    INTEGER NOT NULL CHECK (version > 0)
) STRICT;
