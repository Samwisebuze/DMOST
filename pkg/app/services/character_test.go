package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samwisebuze/dmost/internal/dto/v1alpha"
	"github.com/samwisebuze/dmost/internal/infra/inmem"
	"github.com/samwisebuze/dmost/internal/test"
	"github.com/samwisebuze/dmost/pkg/app/services"
	"github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedCharacter puts one character in a fresh repository and returns both.
func seedCharacter(t *testing.T, sheet json.RawMessage) (*services.CharacterService, *character.Character) {
	t.Helper()
	repo := inmem.NewCharacterRepository()
	chr := test.MustCharacter(t, sheet)
	require.NoError(t, repo.Save(context.Background(), chr))
	return services.NewCharacterService(repo), chr
}

func TestCharacterService_Create(t *testing.T) {
	t.Run("stores the sheet verbatim", func(t *testing.T) {
		sut := services.NewCharacterService(inmem.NewCharacterRepository())
		sheet := test.MustCharacterSheet(t)

		got, err := sut.Create(context.Background(), v1alpha.CreateCharacterRequest{Sheet: sheet})
		require.NoError(t, err)

		assert.NotEmpty(t, got.ID())
		assert.JSONEq(t, string(sheet), string(got.Data()))

		found, err := sut.Find(context.Background(), got.ID().String())
		require.NoError(t, err)
		assert.JSONEq(t, string(sheet), string(found.Data()))
	})

	t.Run("accepts a sheet with no _id", func(t *testing.T) {
		// A create request has no identity to state yet — the aggregate's own
		// CharacterID is assigned here, so the schema must not demand one.
		var doc map[string]any
		require.NoError(t, json.Unmarshal(test.MustCharacterSheet(t), &doc))
		delete(doc, "_id")
		sheet, err := json.Marshal(doc)
		require.NoError(t, err)

		sut := services.NewCharacterService(inmem.NewCharacterRepository())
		got, err := sut.Create(context.Background(), v1alpha.CreateCharacterRequest{Sheet: sheet})
		require.NoError(t, err)
		assert.NotEmpty(t, got.ID())
	})

	t.Run("keeps fields the generated schema has no home for", func(t *testing.T) {
		// The sheet is stored as the client's bytes, not as a re-encoding of
		// the generated struct — so an unknown field survives the round trip.
		var doc map[string]any
		require.NoError(t, json.Unmarshal(test.MustCharacterSheet(t), &doc))
		doc["house_rule_notes"] = "crits do max damage"
		sheet, err := json.Marshal(doc)
		require.NoError(t, err)

		sut := services.NewCharacterService(inmem.NewCharacterRepository())
		got, err := sut.Create(context.Background(), v1alpha.CreateCharacterRequest{Sheet: sheet})
		require.NoError(t, err)

		var stored map[string]any
		require.NoError(t, json.Unmarshal(got.Data(), &stored))
		assert.Equal(t, "crits do max damage", stored["house_rule_notes"])
	})

	t.Run("rejects a sheet the schema refuses", func(t *testing.T) {
		sut := services.NewCharacterService(inmem.NewCharacterRepository())

		for name, sheet := range map[string]string{
			"missing required fields": `{"_id":"abc"}`,
			"not an object":           `["nope"]`,
			"not JSON":                `{`,
			"empty":                   ``,
		} {
			t.Run(name, func(t *testing.T) {
				_, err := sut.Create(context.Background(), v1alpha.CreateCharacterRequest{
					Sheet: json.RawMessage(sheet),
				})
				require.ErrorIs(t, err, common.ErrInvalid)
			})
		}
	})
}

func TestCharacterService_Find(t *testing.T) {
	t.Run("returns the stored character", func(t *testing.T) {
		sut, chr := seedCharacter(t, test.MustCharacterSheetNamed(t, "Bruenor"))

		got, err := sut.Find(context.Background(), chr.ID().String())
		require.NoError(t, err)
		assert.Equal(t, chr.ID(), got.ID())
		assert.JSONEq(t, string(chr.Data()), string(got.Data()))
	})

	t.Run("a malformed id is invalid, not missing", func(t *testing.T) {
		sut, _ := seedCharacter(t, test.MustCharacterSheet(t))

		_, err := sut.Find(context.Background(), "not-a-uuid")
		require.ErrorIs(t, err, common.ErrInvalid)
	})

	t.Run("an unknown id is not found", func(t *testing.T) {
		sut, _ := seedCharacter(t, test.MustCharacterSheet(t))

		_, err := sut.Find(context.Background(), character.NewCharacterID().String())
		require.ErrorIs(t, err, common.ErrNotFound)
	})
}

func TestCharacterService_Update(t *testing.T) {
	t.Run("replaces the sheet", func(t *testing.T) {
		sut, chr := seedCharacter(t, test.MustCharacterSheetNamed(t, "Bruenor"))
		next := test.MustCharacterSheetNamed(t, "Catti-brie")

		got, err := sut.Update(context.Background(), chr.ID(), v1alpha.UpdateCharacterRequest{Sheet: next})
		require.NoError(t, err)

		assert.JSONEq(t, string(next), string(got.Data()))
		assert.Equal(t, chr.ID(), got.ID(), "identity rides along on the loaded aggregate")
		assert.Equal(t, chr.CreatedAt(), got.CreatedAt(), "CreatedAt rides along too")

		found, err := sut.Find(context.Background(), chr.ID().String())
		require.NoError(t, err)
		assert.JSONEq(t, string(next), string(found.Data()))
	})

	t.Run("an omitted sheet leaves the stored one unchanged", func(t *testing.T) {
		sut, chr := seedCharacter(t, test.MustCharacterSheetNamed(t, "Bruenor"))

		got, err := sut.Update(context.Background(), chr.ID(), v1alpha.UpdateCharacterRequest{})
		require.NoError(t, err)
		assert.JSONEq(t, string(chr.Data()), string(got.Data()))
	})

	t.Run("advances the version", func(t *testing.T) {
		sut, chr := seedCharacter(t, test.MustCharacterSheet(t))

		got, err := sut.Update(context.Background(), chr.ID(), v1alpha.UpdateCharacterRequest{
			Sheet: test.MustCharacterSheetNamed(t, "Wulfgar"),
		})
		require.NoError(t, err)
		assert.Equal(t, chr.Version().Next(), got.Version())
	})

	t.Run("a stale version is a conflict", func(t *testing.T) {
		sut, chr := seedCharacter(t, test.MustCharacterSheet(t))
		stale := chr.Version().Uint64()

		_, err := sut.Update(context.Background(), chr.ID(), v1alpha.UpdateCharacterRequest{
			Sheet: test.MustCharacterSheetNamed(t, "Wulfgar"),
		})
		require.NoError(t, err)

		// Second writer still holds the revision it read before the first won.
		_, err = sut.Update(context.Background(), chr.ID(), v1alpha.UpdateCharacterRequest{
			Sheet:   test.MustCharacterSheetNamed(t, "Regis"),
			Version: ptr(stale),
		})
		require.ErrorIs(t, err, common.ErrConflict)
	})

	t.Run("a version no client could have read is invalid", func(t *testing.T) {
		sut, chr := seedCharacter(t, test.MustCharacterSheet(t))

		_, err := sut.Update(context.Background(), chr.ID(), v1alpha.UpdateCharacterRequest{
			Sheet:   test.MustCharacterSheet(t),
			Version: ptr(uint64(0)),
		})
		require.ErrorIs(t, err, common.ErrInvalid)
	})

	t.Run("an invalid sheet does not touch the stored one", func(t *testing.T) {
		sut, chr := seedCharacter(t, test.MustCharacterSheetNamed(t, "Bruenor"))

		_, err := sut.Update(context.Background(), chr.ID(), v1alpha.UpdateCharacterRequest{
			Sheet: json.RawMessage(`{"_id":"abc"}`),
		})
		require.ErrorIs(t, err, common.ErrInvalid)

		found, err := sut.Find(context.Background(), chr.ID().String())
		require.NoError(t, err)
		assert.JSONEq(t, string(chr.Data()), string(found.Data()))
	})

	t.Run("an unknown character is not found", func(t *testing.T) {
		sut, _ := seedCharacter(t, test.MustCharacterSheet(t))

		_, err := sut.Update(context.Background(), character.NewCharacterID(), v1alpha.UpdateCharacterRequest{
			Sheet: test.MustCharacterSheet(t),
		})
		require.ErrorIs(t, err, common.ErrNotFound)
	})
}
