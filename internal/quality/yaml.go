package quality

import (
	"fmt"

	"skillbench/internal/model"
)

// buildCheck dispatches a QualityCheckSpec into a typed Check. Exactly one of
// the optional fields must be set; otherwise we error so cases.yaml typos
// surface immediately.
func buildCheck(spec model.QualityCheckSpec, opts BuildOptions) (Check, error) {
	set := 0
	if spec.Script != "" {
		set++
	}
	if spec.FileExists != "" {
		set++
	}
	if spec.JSONSchema != nil {
		set++
	}
	if spec.Regex != "" {
		set++
	}
	if spec.Similarity != nil {
		set++
	}
	if set == 0 {
		return nil, fmt.Errorf("empty quality check spec")
	}
	if set > 1 {
		return nil, fmt.Errorf("quality check spec has multiple kinds set; pick one")
	}
	switch {
	case spec.Script != "":
		timeout := opts.ScriptTimeout
		if timeout <= 0 {
			timeout = 30
		}
		return ScriptCheck{Path: spec.Script, TimeoutSeconds: timeout}, nil
	case spec.FileExists != "":
		return FileExistsCheck{Path: spec.FileExists}, nil
	case spec.JSONSchema != nil:
		return JSONSchemaCheck{File: spec.JSONSchema.File, Schema: spec.JSONSchema.Schema}, nil
	case spec.Regex != "":
		return RegexCheck{Pattern: spec.Regex}, nil
	case spec.Similarity != nil:
		method := spec.Similarity.Method
		if method == "" {
			method = opts.SimilarityMethod
		}
		if method == "" {
			method = "rouge"
		}
		return SimilarityCheck{
			Reference: spec.Similarity.Reference,
			Method:    method,
			Threshold: spec.Similarity.Threshold,
			LLMRunner: opts.SimilarityRunner,
		}, nil
	}
	return nil, fmt.Errorf("unknown quality check spec")
}
