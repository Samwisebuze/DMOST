package common

import "fmt"

// minVersion is the version a newly constructed aggregate carries — the one it
// holds after the insert that first persists it. Each subsequent replacement
// advances it by one, so the version doubles as a count of the writes an
// aggregate has seen.
const minVersion uint64 = 1

// Version is the revision an aggregate was loaded at, and the mechanism behind
// optimistic locking: a caller that passes its version back to a repository
// gets its write refused with [ErrConflict] if the stored revision has moved
// on, instead of silently overwriting whoever got there first.
//
// It is opaque on purpose. The number means nothing outside the store that
// issued it, and arithmetic on it is never a caller's business — [Version.Next]
// is the only way to reach another revision, and [Aggregate.NextVersion] the
// only way to put one on an aggregate.
//
// The zero Version is the revision of an aggregate that has never been
// persisted; see [Version.IsZero].
type Version struct {
	n uint64
}

// NewVersion returns the revision a freshly constructed aggregate carries.
func NewVersion() Version { return Version{n: minVersion} }

// ParseVersion converts a revision a caller supplied — from a request body,
// say — into a Version. Zero is rejected: it is the revision of an aggregate
// that was never stored, so no caller can have read it.
//
// Repositories reconstructing state they wrote themselves should use the
// rehydrating factories instead, which skip this check.
func ParseVersion(n uint64) (Version, error) {
	if n < minVersion {
		return Version{}, fmt.Errorf("%w: version must be at least %d", ErrInvalid, minVersion)
	}
	return Version{n: n}, nil
}

// RehydrateVersion skips validation. Only repositories should reach it, via the
// rehydrating factories beside each aggregate.
func RehydrateVersion(n uint64) Version { return Version{n: n} }

// Next returns the revision that follows this one. It is pure — computing a
// successor is harmless, and storing one on an aggregate is what needs a
// [github.com/samwisebuze/dmost/pkg/domain/internal/lock.Key].
func (v Version) Next() Version { return Version{n: v.n + 1} }

// Equal reports whether two revisions are the same. Callers should prefer it
// over ==, so the comparison stays correct if the representation changes.
func (v Version) Equal(o Version) bool { return v == o }

// IsZero reports whether this is the revision of an aggregate that has never
// been persisted.
func (v Version) IsZero() bool { return v == Version{} }

// Uint64 unwraps the revision for a wire format to carry. Nothing inside the
// domain should need it.
func (v Version) Uint64() uint64 { return v.n }

func (v Version) String() string { return fmt.Sprintf("%d", v.n) }
