// Package internal anchors the repo-private code
//
// Regenerate with `go generate ./internal/...` from the repo root.
package internal

//go:generate sh -c "go tool go-jsonschema -p character --resolve-extension json -o dto/v1alpha/character/character.gen.go ../pkg/domain/character/schema/*.schema.json"
