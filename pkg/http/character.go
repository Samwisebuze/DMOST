package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/samwisebuze/dmost/internal/dto/v1alpha"
	"github.com/samwisebuze/dmost/internal/dto/v1alpha/mapper"
	"github.com/samwisebuze/dmost/pkg/app"
	"github.com/samwisebuze/dmost/pkg/domain/character"
	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/samwisebuze/dmost/pkg/http/problem"
)

// registerCharacterRoutes wires the Character resource.
//
// There is no collection GET and no DELETE: [character.CharacterRepository]
// exposes Save and Find and nothing else, so [app.CharacterService] has no
// listing or deletion for a handler to call. Adding either starts at the port,
// not here.
func (s *Server) registerCharacterRoutes(router *mux.Router) {
	r := router.PathPrefix("/characters").Subrouter()
	r.Handle("", CreateCharacterHandler(s.app)).Methods(http.MethodPost)
	// PATCH for the same reason /users uses it: a request that omits the sheet
	// leaves the stored one alone rather than clearing it, and PATCH is one of
	// the methods serveHTTP's "_method" form override accepts. That a supplied
	// sheet replaces the stored one *whole* is a fact about the document, not
	// about the method — the domain has no view inside a sheet to merge along.
	r.Handle("/{id}", UpdateCharacterHandler(s.app)).Methods(http.MethodPatch)
	r.Handle("/{id}", FindCharacterHandler(s.app)).Methods(http.MethodGet)
}

func (u urlBuilder) CreateCharacter() string {
	return fmt.Sprintf("%s/characters", u.Server)
}

// Character is the URL of one character: the same path answers GET and PATCH.
//
// A CharacterID is a UUID today, so it holds no dot for serveHTTP's ".json"
// suffix stripping to trip over and nothing PathEscape would alter; the escape
// is there against that ceasing to be true.
func (u urlBuilder) Character(id character.CharacterID) string {
	return fmt.Sprintf("%s/characters/%s", u.Server, url.PathEscape(id.String()))
}

func CreateCharacterHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req v1alpha.CreateCharacterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			problem.New().
				Detail(err.Error()).
				Title("invalid_request").
				Wrap(err).
				Status(http.StatusBadRequest).
				WriteTo(w)
			return
		}

		chr, err := app.CharacterService.Create(r.Context(), req)
		if err != nil {
			// 400 where /users answers 422. The difference is what the body
			// *is*: a create request here carries nothing but the sheet, so a
			// sheet the v1alpha schema refuses is a malformed body — the same
			// failure as the undecodable JSON handled above — rather than a
			// well-formed document that ran into a domain rule the way a
			// duplicate email does.
			WriteError(w, r, err, Status(common.ErrInvalid, http.StatusBadRequest))
			return
		}

		json.NewEncoder(NewWriter(w).
			ContentType(v1alpha.ContentTypeCharacterJSON).
			Status(http.StatusCreated),
		).Encode(mapper.CharacterToResponse(chr))
	}
}

func (c *Client) CreateCharacter(ctx context.Context, req v1alpha.CreateCharacterRequest) (character.Character, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return character.Character{}, fmt.Errorf("POST %q: %w", c.urls.CreateCharacter(), err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.urls.CreateCharacter(), bytes.NewBuffer(raw))
	if err != nil {
		return character.Character{}, fmt.Errorf("POST %q: %w", c.urls.CreateCharacter(), err)
	}
	httpReq.Header.Set("Content-Type", v1alpha.ContentTypeJSON)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return character.Character{}, fmt.Errorf("POST %q failed: %w", c.urls.CreateCharacter(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var prob problem.Problem
		if err := json.NewDecoder(resp.Body).Decode(&prob); err != nil {
			return character.Character{}, fmt.Errorf("POST %q: unprocessable response [code=%q]: %w", c.urls.CreateCharacter(), resp.Status, err)
		}
		return character.Character{}, fmt.Errorf("POST %q: %w", c.urls.CreateCharacter(), &prob)
	}

	data, err := decode[v1alpha.CharacterResponse](resp)
	if err != nil {
		return character.Character{}, fmt.Errorf("POST %q: %w", c.urls.CreateCharacter(), err)
	}

	return mapper.CharacterResponseToCharacter(data), nil
}

func UpdateCharacterHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := character.CharacterID(mux.Vars(r)["id"])

		var req v1alpha.UpdateCharacterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			problem.New().
				Detail(err.Error()).
				Title("invalid_request").
				Wrap(err).
				Status(http.StatusBadRequest).
				WriteTo(w)
			return
		}

		chr, err := app.CharacterService.Update(r.Context(), id, req)
		if err != nil {
			// A sheet is a malformed body here for the same reason it is on
			// create; not-found and conflict take the defaults. The override
			// only reaches ErrInvalid, so a lost race still answers 409 rather
			// than a 400 the client would think it could fix.
			WriteError(w, r, err, Status(common.ErrInvalid, http.StatusBadRequest))
			return
		}

		res := mapper.CharacterToResponse(chr)
		w.Header().Set("Content-Type", v1alpha.ContentTypeCharacterJSON)
		json.NewEncoder(w).Encode(res)
	}
}

func (c *Client) UpdateCharacter(ctx context.Context, id character.CharacterID, req v1alpha.UpdateCharacterRequest) (character.Character, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return character.Character{}, fmt.Errorf("PATCH %q: %w", c.urls.Character(id), err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.urls.Character(id), bytes.NewBuffer(raw))
	if err != nil {
		return character.Character{}, fmt.Errorf("PATCH %q: %w", c.urls.Character(id), err)
	}
	httpReq.Header.Set("Content-Type", v1alpha.ContentTypeJSON)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return character.Character{}, fmt.Errorf("PATCH %q failed: %w", c.urls.Character(id), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var prob problem.Problem
		if err := json.NewDecoder(resp.Body).Decode(&prob); err != nil {
			return character.Character{}, fmt.Errorf("PATCH %q: unprocessable response [code=%q]: %w", c.urls.Character(id), resp.Status, err)
		}
		return character.Character{}, fmt.Errorf("PATCH %q: %w", c.urls.Character(id), &prob)
	}

	data, err := decode[v1alpha.CharacterResponse](resp)
	if err != nil {
		return character.Character{}, fmt.Errorf("PATCH %q: %w", c.urls.Character(id), err)
	}

	return mapper.CharacterResponseToCharacter(data), nil
}

func FindCharacterHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := mux.Vars(r)["id"]
		if !ok || id == "" {
			problem.New().
				Detail("the character route is registered without an {id} variable").
				Of(http.StatusInternalServerError).
				WriteTo(w)
			return
		}

		// The ID goes through unparsed: [app.CharacterService.Find] parses it,
		// so a malformed one is one ErrInvalid from the layer that owns the
		// rule rather than a second check drifting out of step here.
		chr, err := app.CharacterService.Find(r.Context(), id)
		if err != nil {
			// An ID that will not parse is a malformed request line, not a
			// document that broke a rule, so ErrInvalid is a 400 here too.
			WriteError(w, r, err, Status(common.ErrInvalid, http.StatusBadRequest))
			return
		}

		res := mapper.CharacterToResponse(chr)
		w.Header().Set("Content-Type", v1alpha.ContentTypeCharacterJSON)
		json.NewEncoder(w).Encode(res)
	}
}

func (c *Client) FindCharacter(ctx context.Context, id character.CharacterID) (character.Character, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.urls.Character(id), nil)
	if err != nil {
		return character.Character{}, fmt.Errorf("GET %q: %w", c.urls.Character(id), err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return character.Character{}, fmt.Errorf("GET %q failed: %w", c.urls.Character(id), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var prob problem.Problem
		if err := json.NewDecoder(resp.Body).Decode(&prob); err != nil {
			return character.Character{}, fmt.Errorf("GET %q: unprocessable response [code=%q]: %w", c.urls.Character(id), resp.Status, err)
		}
		return character.Character{}, fmt.Errorf("GET %q: %w", c.urls.Character(id), &prob)
	}

	data, err := decode[v1alpha.CharacterResponse](resp)
	if err != nil {
		return character.Character{}, fmt.Errorf("GET %q: %w", c.urls.Character(id), err)
	}

	return mapper.CharacterResponseToCharacter(data), nil
}
