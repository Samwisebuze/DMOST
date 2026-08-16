package http_test

import (
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/samwisebuze/dmost/pkg/http"
	"github.com/samwisebuze/dmost/pkg/http/problem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeError runs [http.WriteError] over a recorder and hands back the response
// with its body decoded, which is all every case below needs.
func writeError(t *testing.T, err error, overrides ...http.ErrorRule) (*nethttp.Response, map[string]any) {
	t.Helper()

	w := httptest.NewRecorder()
	http.WriteError(w, httptest.NewRequest(nethttp.MethodGet, "/whatever", nil), err, overrides...)

	resp := w.Result()
	body := map[string]any{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return resp, body
}

func TestWriteError_MapsDomainSentinelsByDefault(t *testing.T) {
	tests := map[string]struct {
		err        error
		wantStatus int
	}{
		"missing resource": {common.ErrNotFound, nethttp.StatusNotFound},
		"lost update":      {common.ErrConflict, nethttp.StatusConflict},
		"broken rule":      {common.ErrInvalid, nethttp.StatusUnprocessableEntity},
		// ErrExists wraps ErrInvalid, so it must land on the same status
		// without the table naming it.
		"duplicate": {common.ErrExists, nethttp.StatusUnprocessableEntity},
		// Wrapped by a service the way a real one reports it, rather than
		// handed over bare.
		"wrapped by a caller": {fmt.Errorf("find user: %w", common.ErrNotFound), nethttp.StatusNotFound},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			resp, _ := writeError(t, tc.err)

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			assert.Equal(t, problem.ContentTypeJSON, resp.Header.Get("Content-Type"))
		})
	}
}

func TestWriteError_PrefersAnOverrideToTheDefault(t *testing.T) {
	resp, _ := writeError(t, common.ErrInvalid, http.Status(common.ErrInvalid, nethttp.StatusBadRequest))

	assert.Equal(t, nethttp.StatusBadRequest, resp.StatusCode)
}

// An override on a broad sentinel has to reach everything wrapping it, or
// /characters would answer 400 for a bare ErrInvalid and 422 for an ErrExists.
func TestWriteError_OverrideCatchesWrappingSentinels(t *testing.T) {
	resp, _ := writeError(t, common.ErrExists, http.Status(common.ErrInvalid, nethttp.StatusBadRequest))

	assert.Equal(t, nethttp.StatusBadRequest, resp.StatusCode)
}

// The point of overriding one sentinel is that the rest keep their defaults —
// /characters remaps ErrInvalid and must still answer a lost race with 409.
func TestWriteError_LeavesUnnamedSentinelsOnTheirDefault(t *testing.T) {
	resp, _ := writeError(t, common.ErrConflict, http.Status(common.ErrInvalid, nethttp.StatusBadRequest))

	assert.Equal(t, nethttp.StatusConflict, resp.StatusCode)
}

func TestWriteError_MatchesOverridesInOrder(t *testing.T) {
	resp, _ := writeError(t, common.ErrExists,
		http.Status(common.ErrExists, nethttp.StatusConflict),
		http.Status(common.ErrInvalid, nethttp.StatusBadRequest),
	)

	assert.Equal(t, nethttp.StatusConflict, resp.StatusCode, "the narrower rule listed first must win")
}

func TestWriteError_ExposesTheReasonOnAClientError(t *testing.T) {
	_, body := writeError(t, fmt.Errorf("handle taken: %w", common.ErrExists))

	assert.Equal(t, "handle taken: invalid: resource exists", body["reason"],
		"a 4xx tells the client what to fix")
}

func TestWriteError_KeepsTheCauseServerSideOnAnUnrecognisedError(t *testing.T) {
	resp, body := writeError(t, fmt.Errorf("dial tcp 10.0.0.1:5432: connection refused"))

	assert.Equal(t, nethttp.StatusInternalServerError, resp.StatusCode)
	assert.NotContains(t, body, "reason", "an unrecognised failure is as likely to be infrastructure as a domain rule")
}

// Calling this with no error is a handler bug, and 200 with an empty body is
// the one answer that cannot be right: the handler already decided it was not
// writing a resource.
func TestWriteError_AnswersServerErrorWhenGivenNoError(t *testing.T) {
	resp, _ := writeError(t, nil)

	assert.Equal(t, nethttp.StatusInternalServerError, resp.StatusCode)
}
