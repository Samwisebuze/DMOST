package http_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/samwisebuze/dmost/pkg/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_ReturnsOK(t *testing.T) {
	handler := http.HealthHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/healthz", nil)
	handler(w, r)

	resp := w.Result()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body http.HealthResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, http.HealthStatus, body.Status)
}
