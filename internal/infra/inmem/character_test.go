package inmem_test

import (
	"testing"

	"github.com/samwisebuze/dmost/internal/infra/inmem"
	"github.com/samwisebuze/dmost/internal/test/repotest"
	"github.com/samwisebuze/dmost/pkg/domain/character"
)

// The scenarios themselves live in
// [github.com/samwisebuze/dmost/internal/test/repotest], because the rules they
// describe belong to the port rather than to this implementation, and a second
// backend now has to satisfy the same ones. This package's Save is still the
// reference implementation of those rules — see the package doc — but it is no
// longer the only place they are written down.
func TestCharacterRepository_Contract(t *testing.T) {
	repotest.RunCharacterRepositoryContract(t, func(t *testing.T) character.CharacterRepository {
		return inmem.NewCharacterRepository()
	})
}
