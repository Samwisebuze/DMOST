package http

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/samwisebuze/dmost/pkg/session"
)

// SetLocalSession registers the ephemeral local-table API before Open.
func (s *Server) SetLocalSession(table *session.Session, hostToken string) {
	s.localSession = table
	s.hostToken = hostToken
	api := s.router.PathPrefix("/api/local/v1").Subrouter()
	api.HandleFunc("/join", s.localJoin).Methods(http.MethodPost)
	api.HandleFunc("/state", s.localState).Methods(http.MethodGet)
	api.HandleFunc("/messages", s.localSend).Methods(http.MethodPost)
	api.HandleFunc("/events", s.localEvents).Methods(http.MethodGet)
	api.HandleFunc("/end", s.localEnd).Methods(http.MethodPost)
	s.registerLocalWeb()
}

func (s *Server) localJoin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if !readLocalJSON(w, r, &in) {
		return
	}
	token, p, err := s.localSession.Join(in.Code, in.Name)
	if err != nil {
		localError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "dmost_local", Value: token, Path: "/api/local/v1/events", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode})
	localJSON(w, http.StatusOK, map[string]any{"token": token, "participant": p})
}

func (s *Server) localState(w http.ResponseWriter, r *http.Request) {
	ps, seq, err := s.localSession.Snapshot(localToken(r))
	if err != nil {
		localError(w, err)
		return
	}
	localJSON(w, http.StatusOK, map[string]any{"participants": ps, "sequence": seq})
}

func (s *Server) localSend(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text string `json:"text"`
	}
	if !readLocalJSON(w, r, &in) {
		return
	}
	m, err := s.localSession.Send(localToken(r), in.Text)
	if err != nil {
		localError(w, err)
		return
	}
	localJSON(w, http.StatusOK, m)
}

func (s *Server) localEvents(w http.ResponseWriter, r *http.Request) {
	last := r.URL.Query().Get("after")
	if last == "" {
		last = r.Header.Get("Last-Event-ID")
	}
	var after uint64
	if last != "" {
		var err error
		after, err = strconv.ParseUint(last, 10, 64)
		if err != nil {
			localJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_sequence", "message": "Invalid event sequence"})
			return
		}
	}
	replay, events, cancel, err := s.localSession.Subscribe(localToken(r), after)
	if err != nil {
		localError(w, err)
		return
	}
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		localJSON(w, http.StatusInternalServerError, map[string]string{"error": "stream_unavailable"})
		return
	}
	for _, e := range replay {
		if !writeEvent(w, flusher, e) {
			return
		}
	}
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-events:
			if !ok {
				return
			}
			if !writeEvent(w, flusher, e) || e.Type == "ended" {
				return
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) localEnd(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")), []byte(s.hostToken)) != 1 || s.hostToken == "" {
		localJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_host_token", "message": "Host token required"})
		return
	}
	s.localSession.End()
	localJSON(w, http.StatusOK, map[string]bool{"ended": true})
}

func readLocalJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		localJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json", "message": "Invalid JSON request"})
		return false
	}
	return true
}

func localToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if c, err := r.Cookie("dmost_local"); err == nil {
		return c.Value
	}
	return ""
}

func localError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, session.ErrCode):
		status, code = http.StatusForbidden, "invalid_code"
	case errors.Is(err, session.ErrToken):
		status, code = http.StatusUnauthorized, "invalid_token"
	case errors.Is(err, session.ErrName):
		status, code = http.StatusBadRequest, "invalid_name"
	case errors.Is(err, session.ErrMessage):
		status, code = http.StatusBadRequest, "invalid_message"
	case errors.Is(err, session.ErrFull):
		status, code = http.StatusConflict, "session_full"
	case errors.Is(err, session.ErrReplayExpired):
		status, code = http.StatusConflict, "replay_expired"
	case errors.Is(err, session.ErrEnded):
		status, code = http.StatusGone, "session_ended"
	}
	localJSON(w, status, map[string]string{"error": code, "message": strings.ReplaceAll(code, "_", " ")})
}

func localJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeEvent(w http.ResponseWriter, f http.Flusher, e session.Event) bool {
	data, err := json.Marshal(e)
	if err != nil {
		return false
	}
	if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.Sequence, e.Type, data); err != nil {
		return false
	}
	f.Flush()
	return true
}
