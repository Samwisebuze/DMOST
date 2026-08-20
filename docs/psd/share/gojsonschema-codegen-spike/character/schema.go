package character

// The compiled schema — the authority the two hand-written files above defer
// to. It is compiled from the *same* files the generator reads, embedded so
// the binary carries them (the release image is scratch; a source/file loader
// would find nothing at runtime).

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed schema/*.schema.json
var schemaFS embed.FS

// schemaBase is the $id prefix the 14 files declare. Every cross-file $ref is
// relative to it, so each file is registered under this prefix and nothing is
// ever fetched over the network — example.com is a placeholder, not a host.
const schemaBase = "https://example.com/schemas/srd-5.2.1/"

var (
	compileOnce sync.Once
	compiled    *jsonschema.Schema
	compileErr  error
)

// Schema returns the compiled character schema.
func Schema() (*jsonschema.Schema, error) {
	compileOnce.Do(func() {
		compiler := jsonschema.NewCompiler()
		compiler.Draft = jsonschema.Draft2020

		entries, err := schemaFS.ReadDir("schema")
		if err != nil {
			compileErr = err

			return
		}

		for _, entry := range entries {
			data, err := schemaFS.ReadFile("schema/" + entry.Name())
			if err != nil {
				compileErr = err

				return
			}

			if err := compiler.AddResource(schemaBase+entry.Name(), bytes.NewReader(data)); err != nil {
				compileErr = fmt.Errorf("add %s: %w", entry.Name(), err)

				return
			}
		}

		compiled, compileErr = compiler.Compile(schemaBase + "character.schema.json")
	})

	return compiled, compileErr
}

// Validate checks a raw document against the compiled schema.
func Validate(doc []byte) error {
	sch, err := Schema()
	if err != nil {
		return err
	}

	var v interface{}

	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()

	if err := dec.Decode(&v); err != nil {
		return err
	}

	return sch.Validate(v)
}

// ValidateAt checks a fragment against one $def, so a test can exercise a
// subschema without wrapping it in a whole character.
func ValidateAt(ref string, doc []byte) error {
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020

	entries, err := schemaFS.ReadDir("schema")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		data, err := schemaFS.ReadFile("schema/" + entry.Name())
		if err != nil {
			return err
		}

		if err := compiler.AddResource(schemaBase+entry.Name(), bytes.NewReader(data)); err != nil {
			return err
		}
	}

	sch, err := compiler.Compile(schemaBase + ref)
	if err != nil {
		return err
	}

	var v interface{}

	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()

	if err := dec.Decode(&v); err != nil {
		return err
	}

	return sch.Validate(v)
}
