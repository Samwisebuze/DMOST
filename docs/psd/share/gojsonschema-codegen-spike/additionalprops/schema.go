// Package additionalprops pins the generator's handling of `additionalProperties`.
//
// It exists because the answer is not one answer: it depends on whether the
// object also declares `properties`, and two of the six combinations produce Go
// that does not round-trip. See additionalprops_test.go for the table.
//
// The fixture is self-contained (one file, no cross-file $refs), unlike the
// character schemas, so compiling it needs none of character/schema.go's
// multi-resource loader.
package additionalprops

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed schema/additional-props.schema.json
var schemaJSON []byte

const schemaURL = "https://example.com/spike/additional-props.schema.json"

var (
	compileOnce sync.Once
	compiled    *jsonschema.Schema
	compileErr  error
)

// Schema returns the compiled fixture schema.
func Schema() (*jsonschema.Schema, error) {
	compileOnce.Do(func() {
		compiler := jsonschema.NewCompiler()
		compiler.Draft = jsonschema.Draft2020

		if err := compiler.AddResource(schemaURL, bytes.NewReader(schemaJSON)); err != nil {
			compileErr = err

			return
		}

		compiled, compileErr = compiler.Compile(schemaURL)
	})

	return compiled, compileErr
}

// ValidateAt checks a fragment against one $def, so each variant can be
// exercised on its own.
func ValidateAt(def string, doc []byte) error {
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020

	if err := compiler.AddResource(schemaURL, bytes.NewReader(schemaJSON)); err != nil {
		return err
	}

	sch, err := compiler.Compile(schemaURL + "#/$defs/" + def)
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
