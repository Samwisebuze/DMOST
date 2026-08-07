// Package dto is the depository for this project's wire types: the structs
// that get serialized to and from the network, and nothing else.
//
// Each subdirectory is one version of the wire contract (v1alpha today), and
// each version owns three things:
//
//   - the request and response structs, carrying the `json:` tags that fix
//     their on-the-wire shape;
//   - the versioned media types that name them, e.g.
//     "application/vnd.dmost.user.v1alpha+json";
//   - a mapper subpackage holding every translation between those structs and
//     [github.com/samwisebuze/dmost/pkg/domain].
//
// The wire shapes are deliberately not the domain shapes. v1alpha spells a
// user's name as one flat "First Last" string that the domain splits into
// first and last, and calls "username" what the domain calls a handle. That
// gap is the point: the contract can be ugly, flat, or historical without the
// domain inheriting any of it. Domain types must never grow `json:` tags, and
// no type declared here may escape into the domain — the mappers are the only
// bridge, so a wire change stops at that boundary.
//
// Versions are append-only. A shipped version is frozen the moment a client
// depends on it; a contract change means a new sibling package, not an edit to
// the old one. Two versions coexisting is normal and is what lets clients
// migrate on their own schedule.
//
// These types are JSON-specific despite the format-neutral name. Their shapes
// encode JSON decisions — a combined name string, timestamps as RFC3339 text.
// A second serialization format does not reuse them: gRPC would use generated
// .pb.go types, XML would want its own shapes on an independent version line.
// If that day comes, add a format axis (dto/json/v1alpha, dto/xml/v1alpha)
// rather than hanging `xml:` tags off these structs.
//
// The package lives under internal/ because these versions are pre-release.
// Publishing v1alpha would promise stability the alpha label explicitly
// withholds; keeping it here means it is importable across this repository —
// including the nested cmd/dmostd module, since internal is enforced by import
// path prefix — and by nobody else. A version that stabilizes graduates by
// moving out to pkg/dto.
package dto
