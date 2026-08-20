package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/kaptinlin/jsonschema"
	"github.com/samwisebuze/dmost/pkg/domain/common"
)

func Validate(data json.RawMessage) error {
	trimmed := bytes.TrimSpace(data)
	// Emptiness is checked here rather than left to the schema: the parser
	// reports "" the same way it reports genuine syntax errors, and a caller
	// that sent no sheet at all deserves to be told that instead.
	if len(trimmed) == 0 {
		return fmt.Errorf("%w: character sheet required", common.ErrInvalid)
	}

	result := sheetSchema.ValidateJSON(data)
	if result.IsValid() {
		return nil
	}
	for _, evalErr := range result.Errors {
		if evalErr.Code == codeInvalidJSON {
			return fmt.Errorf("%w: character sheet must be valid JSON", common.ErrInvalid)
		}
	}
	// A bare `4`, `"x"`, `[]`, or `null` fails the schema's own `type`, but
	// saying so in schema terms buries the one thing the caller needs to hear.
	// The parse has already succeeded, so the first byte is decisive.
	if trimmed[0] != '{' {
		return fmt.Errorf("%w: character sheet must be a JSON object", common.ErrInvalid)
	}
	return fmt.Errorf("%w: character sheet does not match the v1alpha schema: %s",
		common.ErrInvalid, strings.Join(sheetViolations(result), "; "))
}

// codeInvalidJSON is the [jsonschema.EvaluationError.Code] the library reports
// when the instance never parsed, as opposed to parsing and then failing a
// keyword. Keying on the code rather than the "format" map key keeps this from
// colliding with a real `format` assertion in the schema.
const codeInvalidJSON = "invalid_json"

// maxReportedViolations bounds the `reason` a rejected sheet comes back with. A
// sheet that is wrong in fifty places is wrong; listing all fifty turns a 400
// body into a wall the caller has to read twice to find the first real problem.
const maxReportedViolations = 5

// sheetSchema is the compiled v1alpha character schema — the whole thing, not a
// stand-in for it. It is assembled from
// [github.com/samwisebuze/dmost/pkg/domain/character/schema/v1alpha], which
// embeds the same documents go-jsonschema generates the wire types from, so the
// rule the domain enforces and the shape the DTOs encode cannot drift.
//
// Two things fall out of using the real schema here. The sheet is no longer
// opaque to the domain: a Character cannot exist unless it satisfies every
// required field, at every depth. And the parse got stricter along the way —
// [jsonschema.Schema.ValidateJSON] decodes under the JSON v2 rules, which
// reject documents [json.Valid] accepts (duplicate object keys, invalid UTF-8
// in strings, trailing bytes after the top-level value), and those are exactly
// the documents whose meaning depends on which decoder reads them back.
var sheetSchema = mustCompileSheetSchema()

// mustCompileSheetSchema compiles the embedded documents as one batch.
//
// The batch is the point: character.schema.json is only an assembly, and its
// $refs reach into the sibling files. Compiling it alone leaves them dangling.
// Each document declares its own absolute $id, so the map keys here only have
// to be distinct — the compiler resolves the refs through the $ids, and the
// relative ones against them.
//
// It panics rather than returning an error: the input is embedded at build
// time, so a failure is a broken build, not something a caller did.
func mustCompileSheetSchema() *jsonschema.Schema {
	files, err := documents.ReadDir(".")
	if err != nil {
		panic(fmt.Errorf("character: reading embedded schema: %w", err))
	}

	batch := make(map[string][]byte, len(files))
	for _, file := range files {
		document, err := documents.ReadFile(file.Name())
		if err != nil {
			panic(fmt.Errorf("character: reading embedded schema %s: %w", file.Name(), err))
		}
		batch[file.Name()] = document
	}

	compiled, err := jsonschema.NewCompiler().CompileBatch(batch)
	if err != nil {
		panic(fmt.Errorf("character: compiling embedded schema: %w", err))
	}
	root, ok := compiled[_root]
	if !ok {
		panic(fmt.Errorf("character: embedded schema has no %s", _root))
	}
	if unresolved := root.UnresolvedReferenceURIs(); len(unresolved) > 0 {
		// Every $ref must have landed. A dangling one does not fail the
		// compile — it fails later, as a sheet that validates against a rule
		// that was never loaded.
		panic(fmt.Errorf("character: unresolved schema references: %s", strings.Join(unresolved, ", ")))
	}
	return root
}

// applicatorKeywords are the schema keywords that only delegate. Their failure
// message says a subschema did not match, which the subschema's own error
// already said with the detail attached — reporting both is noise.
var applicatorKeywords = map[string]bool{
	"$dynamicRef": true, "$ref": true, "additionalProperties": true,
	"allOf": true, "anyOf": true, "contains": true, "dependentSchemas": true,
	"else": true, "if": true, "items": true, "not": true, "oneOf": true,
	"patternProperties": true, "prefixItems": true, "properties": true,
	"propertyNames": true, "then": true,
}

// sheetViolations renders a failed evaluation as `location: reason` lines,
// sorted for a stable message and capped at [maxReportedViolations].
//
// [jsonschema.EvaluationResult.DetailedErrors] hands back every error the
// evaluation collected, at every depth, keyed by the instance path and the
// keyword that failed there — `/abilities/strength/type`. Splitting that key on
// its last segment is what turns the root's own "Property 'abilities' does not
// match the schema" into something that names the field.
func sheetViolations(result *jsonschema.EvaluationResult) []string {
	violations := make([]string, 0, len(result.Errors))
	for key, message := range result.DetailedErrors() {
		location, keyword := "/", key
		if cut := strings.LastIndex(key, "/"); cut >= 0 {
			location, keyword = key[:cut], key[cut+1:]
			if location == "" {
				location = "/"
			}
		}
		if applicatorKeywords[keyword] {
			continue
		}
		violations = append(violations, fmt.Sprintf("%s: %s", location, message))
	}

	sort.Strings(violations)
	if len(violations) > maxReportedViolations {
		return append(violations[:maxReportedViolations:maxReportedViolations],
			fmt.Sprintf("(and %d more)", len(violations)-maxReportedViolations))
	}
	return violations
}
