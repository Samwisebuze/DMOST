package main_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoutes_MethodsAreRegistered drives each route with its own method and
// asserts none of them answers 405.
//
// Nothing else in either module covers route registration: the pkg/http tests
// call handler functions directly with a mux.SetURLVars request, so they pass
// whether or not the route exists, and the typed http.Client can only reach a
// method its own code already sends. A handler wired to the wrong verb — or a
// client sending one the router does not accept — therefore shows up nowhere
// until an e2e test fails for a reason that looks like something else.
//
// The status the route *does* answer is deliberately not asserted; that is
// every other test's job. This one only asks whether the method got through.
func TestRoutes_MethodsAreRegistered(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	base := m.HTTPServer.URL()

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/healthz"},

		{http.MethodPost, "/users"},
		{http.MethodGet, "/users"},
		{http.MethodPatch, "/users/01920000-0000-7000-8000-000000000001"},
		{http.MethodGet, "/users/01920000-0000-7000-8000-000000000001"},

		{http.MethodPost, "/characters"},
		{http.MethodPut, "/characters/01920000-0000-7000-8000-000000000001"},
		{http.MethodPatch, "/characters/01920000-0000-7000-8000-000000000001"},
		{http.MethodGet, "/characters/01920000-0000-7000-8000-000000000001"},
	}

	for _, tc := range tests {
		// Not t.Parallel(): a parallel subtest resumes only after its parent
		// returns, by which point the deferred MustCloseMain has taken the
		// server down and every request fails on dial instead of on status.
		// One server, nine trivial requests — sequential is fine.
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			// An empty JSON object rather than no body: a handler that decodes
			// first would answer 400 either way, but this keeps the failure
			// about the route rather than about the payload.
			req, err := http.NewRequestWithContext(context.Background(), tc.method, base+tc.path, bytes.NewBufferString("{}"))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// 405 is the only status worth asserting on. It is what gorilla
			// answers when a path matches and the method does not, so it means
			// exactly "this verb is not wired" and nothing else. 404 cannot be
			// used the same way: the IDs above name nothing, so a handler that
			// *is* wired answers 404 too, and the two are indistinguishable
			// from out here.
			assert.NotEqual(t, http.StatusMethodNotAllowed, resp.StatusCode,
				"%s %s is not registered — a handler and its client disagree about the verb", tc.method, tc.path)
		})
	}
}
