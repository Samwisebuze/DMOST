package http

import (
	"io"
	"net/http"
)

type writer struct {
	http.ResponseWriter
	status      int
	contentType string
}

var _ io.Writer = (*writer)(nil)

// Write implements [io.Writer].
func (w *writer) Status(code int) *writer {
	w.status = code
	return w
}
func (w *writer) ContentType(typ string) *writer {
	w.contentType = typ
	return w
}

// Write implements [io.Writer].
func (w *writer) Write(p []byte) (n int, err error) {
	if w.contentType != "" {
		w.ResponseWriter.Header().Set("Content-Type", w.contentType)
	}
	if w.status != 0 {
		w.ResponseWriter.WriteHeader(w.status)
	}
	return w.ResponseWriter.Write(p)
}

func NewWriter(w http.ResponseWriter) *writer {
	return &writer{ResponseWriter: w}
}
