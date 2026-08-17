// Package jsonmerge applies JSON Merge Patch documents (RFC 7396).
//
// The algorithm is deliberately format-level and knows nothing about any
// particular document: objects merge recursively, a null value deletes the key
// it names, and everything else — including arrays — replaces what it lands
// on. What a *legal* patch is for a given resource is the caller's rule, not
// this package's; see internal/dto/v1alpha/mapper for the character sheet's.
//
// It lives under internal/ because it is an implementation detail rather than
// a published API. Nothing here promises stability to an outside caller.
package jsonmerge

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Apply merges patch into target and returns the result.
//
// The result is re-encoded from a decoded value, so it is not target's bytes
// with an edit spliced in: object keys come back sorted and insignificant
// whitespace is gone. A caller that needs the client's exact bytes must not
// route them through here. Numbers do survive intact — both documents are
// decoded with [json.Decoder.UseNumber], so a literal keeps its own text
// rather than becoming a float64 and re-encoding in scientific notation.
//
// Errors report a document that would not decode or would not re-encode.
// They are returned bare: whether a malformed patch is a client's fault is a
// question for the layer that accepted it, so wrapping it in a sentinel here
// would be guessing.
func Apply(target, patch json.RawMessage) (json.RawMessage, error) {
	var t any
	if err := decode(target, &t); err != nil {
		return nil, fmt.Errorf("decode target: %w", err)
	}

	var p any
	if err := decode(patch, &p); err != nil {
		return nil, fmt.Errorf("decode patch: %w", err)
	}

	return encode(merge(t, p))
}

// merge is RFC 7396 section 2, transcribed.
//
// It edits target in place, which is safe because the only target it ever
// sees is one Apply decoded a moment ago and owns outright.
func merge(target, patch any) any {
	p, ok := patch.(map[string]any)
	if !ok {
		// A patch that is not an object replaces whatever it is applied to —
		// arrays included, which is why a merge patch cannot address a single
		// element of one.
		return patch
	}

	t, ok := target.(map[string]any)
	if !ok {
		// Patching an object over a non-object (or over a key that was not
		// there) builds a fresh one rather than merging into a value that has
		// no members to merge with.
		t = make(map[string]any, len(p))
	}

	for k, v := range p {
		if v == nil {
			// Null is the delete instruction, and this is the one place the
			// distinction between "absent" and "present and null" matters: a
			// null in the *target* is left alone unless the patch names it.
			delete(t, k)
			continue
		}
		t[k] = merge(t[k], v)
	}

	return t
}

func decode(raw json.RawMessage, v *any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(v)
}

func encode(v any) (json.RawMessage, error) {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	// Without this, a "<" a client stored comes back as < — a different
	// document by byte comparison, for the benefit of an HTML context that
	// never applies to a JSON API response.
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encode result: %w", err)
	}

	// Encode appends a newline; the result is a document, not a stream.
	return json.RawMessage(bytes.TrimRight(buf.Bytes(), "\n")), nil
}
