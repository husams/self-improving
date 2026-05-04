package quality

import (
	"context"
	"fmt"
)

// JSONSchemaCheck validates File against Schema. Implementation lands in
// Slice 2 once the santhosh-tekuri/jsonschema dep is approved.
type JSONSchemaCheck struct {
	File   string
	Schema string
}

func (JSONSchemaCheck) Name() string { return "json_schema" }

func (j JSONSchemaCheck) Run(_ context.Context, _ Input) Result {
	return Result{
		Name:   "json_schema",
		Score:  0,
		Passed: false,
		Note:   fmt.Sprintf("json_schema check not yet implemented (file=%s schema=%s)", j.File, j.Schema),
	}
}
