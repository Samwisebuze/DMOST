package http

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/samwisebuze/dmost/pkg/domain/common"
	"github.com/samwisebuze/dmost/pkg/http/problem"
)

// defaultErrorStatus is the status every handler answers a domain sentinel with
// unless it says otherwise. It is the whole reason a handler no longer spells
// out a case per sentinel: the mapping is a property of the domain error, not of
// the resource that happened to return it, so it belongs in one table.
//
// The order is load-bearing for the same reason the switches it replaces were
// ordered. ErrExists wraps ErrInvalid and ErrConflict deliberately does not, so
// a conflict must be matched before the ErrInvalid catch-all or a lost update
// would answer as a malformed request the client could fix by editing the body.
//
// ErrInvalid answers 422 rather than 400 because it means a well-formed document
// ran into a domain rule — a duplicate email, say. A body that could not be read
// at all never reaches a service, so it never reaches this table; the handler
// answers that 400 itself at the decode.
var defaultErrorStatus = []ErrorRule{
	{common.ErrNotFound, http.StatusNotFound},
	{common.ErrConflict, http.StatusConflict},
	{common.ErrInvalid, http.StatusUnprocessableEntity},
}

// ErrorRule maps a sentinel to the status a handler answers it with. Build one
// with [Status] and pass it to [WriteError].
type ErrorRule struct {
	sentinel error
	status   int
}

// Status overrides the status one sentinel answers with, for one call to
// [WriteError]. It is for a handler whose resource genuinely disagrees with
// defaultErrorStatus — not for restating a default.
//
// /characters is the standing example: a create request there carries nothing
// but the sheet, so a sheet the v1alpha schema refuses is a malformed body
// rather than a document that ran into a domain rule, and the handler says so:
//
//	WriteError(w, r, err, Status(common.ErrInvalid, http.StatusBadRequest))
func Status(sentinel error, status int) ErrorRule {
	return ErrorRule{sentinel: sentinel, status: status}
}

// WriteError writes err as an RFC 7807 problem document, choosing the status
// from overrides first and defaultErrorStatus after. Anything matching neither
// is a failure the caller cannot act on: it is logged and answered 500 with the
// cause kept server-side, since an unrecognized error is as likely to be a
// connection string as a domain rule.
//
// Overrides are consulted in the order given, before every default, so a rule
// on a broad sentinel shadows the defaults for everything wrapping it — an
// override on ErrInvalid also catches ErrExists. List the narrower sentinel
// first where that matters.
//
// A handler needing more than a different status — a Detail, a Location header,
// a body of its own — still writes that case out by hand and passes the rest
// here. That is the override this is built to leave room for.
func WriteError(w http.ResponseWriter, r *http.Request, err error, overrides ...ErrorRule) {
	if err == nil {
		// Reaching here without an error is a bug in the handler, and the one
		// answer that cannot be right is 200 with an empty body: the handler
		// has already decided it is not writing a resource.
		err = errors.New("handler reported a failure with no error")
	}

	if status, ok := matchErrorRule(overrides, err); ok {
		problem.New().Wrap(err).Of(status).WriteTo(w)
		return
	}
	if status, ok := matchErrorRule(defaultErrorStatus, err); ok {
		problem.New().Wrap(err).Of(status).WriteTo(w)
		return
	}

	slog.Error("http: request failed",
		"method", r.Method,
		"path", r.URL.Path,
		"error", err,
	)
	problem.New().WrapSilent(err).Of(http.StatusInternalServerError).WriteTo(w)
}

func matchErrorRule(rules []ErrorRule, err error) (int, bool) {
	for _, rule := range rules {
		if errors.Is(err, rule.sentinel) {
			return rule.status, true
		}
	}
	return 0, false
}
