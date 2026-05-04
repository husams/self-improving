package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// JSONSchemaCheck validates the JSON document at File against the JSON Schema
// at Schema. Binary: 100 if the document validates, 0 otherwise. Errors
// (missing files, malformed JSON, malformed schema) translate to score 0 +
// note; the run does not abort.
type JSONSchemaCheck struct {
	File   string
	Schema string
}

func (JSONSchemaCheck) Name() string { return "json_schema" }

func (j JSONSchemaCheck) Run(_ context.Context, _ Input) Result {
	if j.File == "" || j.Schema == "" {
		return Result{Name: "json_schema", Score: 0, Passed: false, Note: "file and schema are required"}
	}
	schema, err := jsonschema.Compile(j.Schema)
	if err != nil {
		return Result{Name: "json_schema", Score: 0, Passed: false, Note: fmt.Sprintf("compile schema %s: %v", j.Schema, err)}
	}
	docBytes, err := os.ReadFile(j.File)
	if err != nil {
		return Result{Name: "json_schema", Score: 0, Passed: false, Note: fmt.Sprintf("read %s: %v", j.File, err)}
	}
	var doc any
	if err := json.Unmarshal(docBytes, &doc); err != nil {
		return Result{Name: "json_schema", Score: 0, Passed: false, Note: fmt.Sprintf("parse %s: %v", j.File, err)}
	}
	if err := schema.Validate(doc); err != nil {
		return Result{Name: "json_schema", Score: 0, Passed: false, Note: fmt.Sprintf("%s does not validate: %v", j.File, err)}
	}
	return Result{Name: "json_schema", Score: 100, Passed: true, Note: fmt.Sprintf("%s validates against %s", j.File, j.Schema)}
}
