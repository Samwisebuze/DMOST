package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/samwisebuze/dmost/internal/dto/v1alpha"
	"github.com/samwisebuze/dmost/internal/test"
	"github.com/samwisebuze/dmost/pkg/app"
	"github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/samwisebuze/dmost/pkg/http"
	"github.com/samwisebuze/dmost/pkg/http/problem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FakeCharacterService struct {
	FindFn   func(context.Context, string) (character.Character, error)
	CreateFn func(context.Context, v1alpha.CreateCharacterRequest) (character.Character, error)
	UpdateFn func(context.Context, character.CharacterID, v1alpha.UpdateCharacterRequest) (character.Character, error)
}

var _ app.CharacterService = FakeCharacterService{}

// Find implements [app.CharacterService].
func (f FakeCharacterService) Find(ctx context.Context, id string) (character.Character, error) {
	return f.FindFn(ctx, id)
}

// Create implements [app.CharacterService].
func (f FakeCharacterService) Create(ctx context.Context, r v1alpha.CreateCharacterRequest) (character.Character, error) {
	return f.CreateFn(ctx, r)
}

// Update implements [app.CharacterService].
func (f FakeCharacterService) Update(ctx context.Context, id character.CharacterID, r v1alpha.UpdateCharacterRequest) (character.Character, error) {
	return f.UpdateFn(ctx, id, r)
}

func ptr[T any](v T) *T { return &v }

// sheetWithUnknownField returns a schema-valid sheet carrying a field the
// generated v1alpha type has no home for. Marshalling a map yields bytes that
// are already compact and key-sorted, so a transport that passes them through
// untouched hands them back byte for byte — which is what the assertions below
// rely on.
func sheetWithUnknownField(t testing.TB) json.RawMessage {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(test.MustCharacterSheet(t), &doc))
	doc["house_rule_notes"] = "crits do max damage"
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	return raw
}

// characterRequest builds a request carrying body against one character's URL,
// with the {id} route variable set as the router would have.
func characterRequest(t *testing.T, method string, id character.CharacterID, body []byte) *nethttp.Request {
	t.Helper()
	r := httptest.NewRequest(method, "/characters/"+id.String(), bytes.NewBuffer(body))
	return mux.SetURLVars(r, map[string]string{"id": id.String()})
}

func TestCreateCharacterHandler_MapsErrorsToStatus(t *testing.T) {
	body, err := json.Marshal(v1alpha.CreateCharacterRequest{Sheet: test.MustCharacterSheet(t)})
	require.NoError(t, err)

	tests := map[string]struct {
		err        error
		wantStatus int
	}{
		"success": {nil, nethttp.StatusCreated},
		// A create request carries nothing but the sheet, so a sheet the
		// schema refuses is a malformed body rather than the 422 /users
		// answers with for a well-formed body that broke a domain rule.
		"unschematic sheet":    {common.ErrInvalid, nethttp.StatusBadRequest},
		"wrapping ErrInvalid":  {common.ErrExists, nethttp.StatusBadRequest},
		"unrecognised failure": {assert.AnError, nethttp.StatusInternalServerError},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			var a app.App
			a.CharacterService = FakeCharacterService{
				CreateFn: func(context.Context, v1alpha.CreateCharacterRequest) (character.Character, error) {
					return character.Character{}, tc.err
				},
			}

			w := httptest.NewRecorder()
			http.CreateCharacterHandler(&a)(w, httptest.NewRequest(nethttp.MethodPost, "/characters", bytes.NewBuffer(body)))

			resp := w.Result()
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			if tc.err == nil {
				assert.Equal(t, v1alpha.ContentTypeCharacterJSON, resp.Header.Get("Content-Type"))
				return
			}
			assert.Equal(t, problem.ContentTypeJSON, resp.Header.Get("Content-Type"))
		})
	}
}

func TestCreateCharacterHandler_Returns400OnAnUndecodableBody(t *testing.T) {
	var a app.App

	w := httptest.NewRecorder()
	http.CreateCharacterHandler(&a)(w, httptest.NewRequest(nethttp.MethodPost, "/characters", strings.NewReader("")))

	resp := w.Result()
	assert.Equal(t, nethttp.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, problem.ContentTypeJSON, resp.Header.Get("Content-Type"))
}

func TestCreateCharacterHandler_PassesTheSheetThroughVerbatim(t *testing.T) {
	sheet := sheetWithUnknownField(t)
	body, err := json.Marshal(v1alpha.CreateCharacterRequest{Sheet: sheet})
	require.NoError(t, err)

	var got json.RawMessage
	var a app.App
	a.CharacterService = FakeCharacterService{
		CreateFn: func(_ context.Context, req v1alpha.CreateCharacterRequest) (character.Character, error) {
			got = req.Sheet
			return *test.MustCharacter(t, req.Sheet), nil
		},
	}

	w := httptest.NewRecorder()
	http.CreateCharacterHandler(&a)(w, httptest.NewRequest(nethttp.MethodPost, "/characters", bytes.NewBuffer(body)))

	assert.Equal(t, string(sheet), string(got), "the service must see the client's bytes, not a re-encoding")

	var res v1alpha.CharacterResponse
	require.NoError(t, json.NewDecoder(w.Result().Body).Decode(&res))
	assert.Equal(t, string(sheet), string(res.Sheet), "an unknown field must survive back out to the client")
}

func TestUpdateCharacterHandler_MapsErrorsToStatus(t *testing.T) {
	body, err := json.Marshal(v1alpha.UpdateCharacterRequest{Sheet: test.MustCharacterSheet(t)})
	require.NoError(t, err)

	tests := map[string]struct {
		err        error
		wantStatus int
	}{
		"success":           {nil, nethttp.StatusOK},
		"unknown character": {common.ErrNotFound, nethttp.StatusNotFound},
		// ErrConflict does not wrap ErrInvalid, so it must reach its own arm
		// rather than falling into the 400 below.
		"stale version":        {common.ErrConflict, nethttp.StatusConflict},
		"unschematic sheet":    {common.ErrInvalid, nethttp.StatusBadRequest},
		"wrapping ErrInvalid":  {common.ErrExists, nethttp.StatusBadRequest},
		"unrecognised failure": {assert.AnError, nethttp.StatusInternalServerError},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			var a app.App
			a.CharacterService = FakeCharacterService{
				UpdateFn: func(context.Context, character.CharacterID, v1alpha.UpdateCharacterRequest) (character.Character, error) {
					return character.Character{}, tc.err
				},
			}

			w := httptest.NewRecorder()
			http.UpdateCharacterHandler(&a)(w, characterRequest(t, nethttp.MethodPatch, character.NewCharacterID(), body))

			resp := w.Result()
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			if tc.err == nil {
				assert.Equal(t, v1alpha.ContentTypeCharacterJSON, resp.Header.Get("Content-Type"))
				return
			}
			assert.Equal(t, problem.ContentTypeJSON, resp.Header.Get("Content-Type"))
		})
	}
}

func TestUpdateCharacterHandler_PassesTheIDAndRequestThrough(t *testing.T) {
	id := character.NewCharacterID()
	body, err := json.Marshal(v1alpha.UpdateCharacterRequest{Version: ptr(uint64(3))})
	require.NoError(t, err)

	var gotID character.CharacterID
	var gotReq v1alpha.UpdateCharacterRequest
	var a app.App
	a.CharacterService = FakeCharacterService{
		UpdateFn: func(_ context.Context, id character.CharacterID, req v1alpha.UpdateCharacterRequest) (character.Character, error) {
			gotID, gotReq = id, req
			return character.Character{}, nil
		},
	}

	http.UpdateCharacterHandler(&a)(httptest.NewRecorder(), characterRequest(t, nethttp.MethodPatch, id, body))

	assert.Equal(t, id, gotID)
	require.NotNil(t, gotReq.Version, "the expected version must survive decoding, or every update is unconditional")
	assert.EqualValues(t, 3, *gotReq.Version)
	// Not nil: a nil json.RawMessage encodes as JSON null, so this is what an
	// omitted sheet looks like by the time it has been through the wire. The
	// mapper reads those bytes as "leave the stored sheet alone" — see
	// mapper.sheetOmitted — which is why the transport can pass them straight
	// through rather than special-casing them here.
	assert.JSONEq(t, "null", string(gotReq.Sheet), "an omitted sheet must not arrive as content")
}

func TestUpdateCharacterHandler_Returns400OnAnUndecodableBody(t *testing.T) {
	var a app.App

	w := httptest.NewRecorder()
	http.UpdateCharacterHandler(&a)(w, characterRequest(t, nethttp.MethodPatch, character.NewCharacterID(), nil))

	resp := w.Result()
	assert.Equal(t, nethttp.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, problem.ContentTypeJSON, resp.Header.Get("Content-Type"))
}

func TestFindCharacterHandler_MapsErrorsToStatus(t *testing.T) {
	tests := map[string]struct {
		err        error
		wantStatus int
	}{
		"success":              {nil, nethttp.StatusOK},
		"unknown character":    {common.ErrNotFound, nethttp.StatusNotFound},
		"unparsable id":        {common.ErrInvalid, nethttp.StatusBadRequest},
		"unrecognised failure": {assert.AnError, nethttp.StatusInternalServerError},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			var a app.App
			a.CharacterService = FakeCharacterService{
				FindFn: func(context.Context, string) (character.Character, error) {
					return *test.MustCharacter(t, test.MustCharacterSheet(t)), tc.err
				},
			}

			w := httptest.NewRecorder()
			http.FindCharacterHandler(&a)(w, characterRequest(t, nethttp.MethodGet, character.NewCharacterID(), nil))

			resp := w.Result()
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			if tc.err == nil {
				assert.Equal(t, v1alpha.ContentTypeCharacterJSON, resp.Header.Get("Content-Type"))
				return
			}
			assert.Equal(t, problem.ContentTypeJSON, resp.Header.Get("Content-Type"))
		})
	}
}

func TestFindCharacterHandler_PassesTheIDThroughUnparsed(t *testing.T) {
	// The handler must not pre-validate: parsing belongs to the service, which
	// owns the ErrInvalid a malformed id produces.
	var gotID string
	var a app.App
	a.CharacterService = FakeCharacterService{
		FindFn: func(_ context.Context, id string) (character.Character, error) {
			gotID = id
			return character.Character{}, common.ErrInvalid
		},
	}

	r := httptest.NewRequest(nethttp.MethodGet, "/characters/not-a-uuid", nil)
	r = mux.SetURLVars(r, map[string]string{"id": "not-a-uuid"})
	w := httptest.NewRecorder()
	http.FindCharacterHandler(&a)(w, r)

	assert.Equal(t, "not-a-uuid", gotID)
	assert.Equal(t, nethttp.StatusBadRequest, w.Result().StatusCode)
}

func TestFindCharacterHandler_Returns500WithoutARouteVariable(t *testing.T) {
	var a app.App

	w := httptest.NewRecorder()
	http.FindCharacterHandler(&a)(w, httptest.NewRequest(nethttp.MethodGet, "/characters/", nil))

	assert.Equal(t, nethttp.StatusInternalServerError, w.Result().StatusCode)
}

func TestFindCharacterHandler_ReturnsTheSheetVerbatim(t *testing.T) {
	sheet := sheetWithUnknownField(t)
	var a app.App
	a.CharacterService = FakeCharacterService{
		FindFn: func(context.Context, string) (character.Character, error) {
			return *test.MustCharacter(t, sheet), nil
		},
	}

	w := httptest.NewRecorder()
	http.FindCharacterHandler(&a)(w, characterRequest(t, nethttp.MethodGet, character.NewCharacterID(), nil))

	var res v1alpha.CharacterResponse
	require.NoError(t, json.NewDecoder(w.Result().Body).Decode(&res))
	assert.Equal(t, string(sheet), string(res.Sheet))
	assert.NotZero(t, res.Version, "a response without a version leaves the client unable to make a conditional update stick")
	assert.NotEmpty(t, res.CreatedAt)
}
