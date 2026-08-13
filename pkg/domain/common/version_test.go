package common_test

import (
	"testing"

	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVersion(t *testing.T) {
	t.Parallel()

	v := common.NewVersion()
	assert.EqualValues(t, 1, v.Uint64(), "a new aggregate is at the revision its first insert stores")
	assert.False(t, v.IsZero(), "a constructed version is not the never-persisted zero")
}

func TestParseVersion(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		raw     uint64
		wantErr bool
	}{
		"the first revision": {1, false},
		"a later revision":   {42, false},
		"never persisted":    {0, true},
		"the largest uint64": {^uint64(0), false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := common.ParseVersion(tc.raw)
			if tc.wantErr {
				require.ErrorIs(t, err, common.ErrInvalid)
				assert.True(t, got.IsZero(), "a rejected version must not come back usable")
				return
			}
			require.NoError(t, err)
			assert.EqualValues(t, tc.raw, got.Uint64())
		})
	}
}

func TestVersion_Next(t *testing.T) {
	t.Parallel()

	t.Run("advances by one", func(t *testing.T) {
		assert.Equal(t, common.RehydrateVersion(2), common.NewVersion().Next())
	})

	t.Run("leaves the receiver alone", func(t *testing.T) {
		// Next is a value operation: computing a successor is harmless, which
		// is why it needs no lock.Key. Storing one on an aggregate does.
		v := common.NewVersion()
		_ = v.Next()
		assert.Equal(t, common.NewVersion(), v)
	})
}

func TestVersion_Equal(t *testing.T) {
	t.Parallel()

	v := common.NewVersion()
	assert.True(t, v.Equal(common.NewVersion()), "same revision")
	assert.False(t, v.Equal(v.Next()), "a stale version must not match the one that replaced it")
}

func TestVersion_IsZero(t *testing.T) {
	t.Parallel()

	var unpersisted common.Version
	assert.True(t, unpersisted.IsZero())
	assert.False(t, common.NewVersion().IsZero())
}

func TestVersion_String(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "1", common.NewVersion().String())
	assert.Equal(t, "0", common.Version{}.String())
}
