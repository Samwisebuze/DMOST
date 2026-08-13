package character

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/samwisebuze/dmost/pkg/domain/common"
)

type CharacterID string

func NewCharacterID() CharacterID { return CharacterID(uuid.Must(uuid.NewV7()).String()) }
func ParseCharacterID(raw string) (CharacterID, error) {
	if err := uuid.Validate(raw); err != nil {
		return "", fmt.Errorf("%w: %w", common.ErrInvalid, err)
	}

	return CharacterID(uuid.MustParse(raw).String()), nil
}

func (cid CharacterID) String() string            { return string(cid) }
func (cid CharacterID) Compare(o CharacterID) int { return strings.Compare(string(cid), string(o)) }

// Equal reports whether two IDs identify the same character. Callers should
// prefer it over ==, so the comparison stays correct if the representation
// changes.
func (cid CharacterID) Equal(other CharacterID) bool {
	return cid == other
}
