package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

// HealthStatus is the value of HealthResponse.Status for a healthy server.
const HealthStatus = "ok"

// HealthResponse is the body of GET /healthz.
//
// Unlike the user endpoints this type is declared here rather than in
// pkg/dto, and it carries an unversioned application/json content type.
type HealthResponse struct {
	Status string `json:"status"`
}

func (s *Server) registerHealthRoutes(router *mux.Router) {
	router.Handle("/healthz", HealthHandler()).Methods(http.MethodGet)
}

// HealthHandler reports process liveness: it answers 200 whenever the server is
// able to serve requests at all.
//
// This is deliberately *not* a readiness check, and takes no dependencies for that
// reason. That makes it safe as the restart signal for an orchestrator
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthResponse{Status: HealthStatus})
	}
}

func (u urlBuilder) Health() string {
	return fmt.Sprintf("%s/healthz", u.Server)
}

// Health returns nil if the server reports itself healthy, and an error
// describing why not otherwise.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.urls.Health(), nil)
	if err != nil {
		return fmt.Errorf("GET %q: %w", c.urls.Health(), err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %q failed: %w", c.urls.Health(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %q: unhealthy [code=%q]", c.urls.Health(), resp.Status)
	}

	data, err := decode[HealthResponse](resp)
	if err != nil {
		return fmt.Errorf("GET %q: %w", c.urls.Health(), err)
	}
	if data.Status != HealthStatus {
		return fmt.Errorf("GET %q: unhealthy [status=%q]", c.urls.Health(), data.Status)
	}

	return nil
}
