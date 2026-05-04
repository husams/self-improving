package quality

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const schemaSrc = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["priority", "rationale"],
  "properties": {
    "priority": {"type": "string", "enum": ["P0", "P1", "P2", "P3"]},
    "rationale": {"type": "string"}
  }
}`

func TestJSONSchemaValid(t *testing.T) {
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.json")
	doc := filepath.Join(dir, "doc.json")
	writeFile(t, schema, schemaSrc)
	writeFile(t, doc, `{"priority":"P0","rationale":"revenue impact"}`)
	res := JSONSchemaCheck{File: doc, Schema: schema}.Run(context.Background(), Input{})
	if !res.Passed || res.Score != 100 {
		t.Fatalf("expected pass, got %+v", res)
	}
}

func TestJSONSchemaInvalid(t *testing.T) {
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.json")
	doc := filepath.Join(dir, "doc.json")
	writeFile(t, schema, schemaSrc)
	writeFile(t, doc, `{"priority":"P9"}`) // missing rationale, bad enum
	res := JSONSchemaCheck{File: doc, Schema: schema}.Run(context.Background(), Input{})
	if res.Passed || res.Score != 0 {
		t.Fatalf("expected fail, got %+v", res)
	}
}

func TestJSONSchemaMissingFile(t *testing.T) {
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.json")
	writeFile(t, schema, schemaSrc)
	res := JSONSchemaCheck{File: filepath.Join(dir, "nope.json"), Schema: schema}.Run(context.Background(), Input{})
	if res.Passed || res.Score != 0 {
		t.Fatalf("expected error→0, got %+v", res)
	}
}

func TestJSONSchemaMalformedDoc(t *testing.T) {
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.json")
	doc := filepath.Join(dir, "doc.json")
	writeFile(t, schema, schemaSrc)
	writeFile(t, doc, `not json{`)
	res := JSONSchemaCheck{File: doc, Schema: schema}.Run(context.Background(), Input{})
	if res.Passed || res.Score != 0 {
		t.Fatalf("expected error→0, got %+v", res)
	}
}

func TestJSONSchemaMalformedSchema(t *testing.T) {
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.json")
	doc := filepath.Join(dir, "doc.json")
	writeFile(t, schema, `not a schema`)
	writeFile(t, doc, `{}`)
	res := JSONSchemaCheck{File: doc, Schema: schema}.Run(context.Background(), Input{})
	if res.Passed || res.Score != 0 {
		t.Fatalf("expected error→0, got %+v", res)
	}
}
