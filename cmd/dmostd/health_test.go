package main_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthAPI_Healthy(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	cli := MustClient(t, m.HTTPServer.URL())

	require.NoError(t, cli.Health(context.Background()))
}

// The .json suffix is stripped by Server.serveHTTP rather than by the router, so
// a probe URL ending in .json has to keep resolving to the same route.
func TestHealthAPI_JSONSuffix(t *testing.T) {
	t.Parallel()

	m := MustRunMain(t)
	defer MustCloseMain(t, m)

	resp, err := http.Get(m.HTTPServer.URL() + "/healthz.json")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
}
