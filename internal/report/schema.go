package report

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed report-v1.schema.json
var schemaBytes []byte

var (
	compileSchemaOnce sync.Once
	compiledSchema    *jsonschema.Schema
	compiledSchemaErr error
)

func Schema() []byte { return append([]byte(nil), schemaBytes...) }

// Validate checks the canonical serialized representation against the bundled
// schema so model and machine contract cannot drift silently.
func Validate(value Report) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal report for validation: %w", err)
	}
	return ValidateJSON(data)
}

func ValidateJSON(data []byte) error {
	schema, err := loadSchema()
	if err != nil {
		return err
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("decode report for schema validation: %w", err)
	}
	if err := schema.Validate(value); err != nil {
		return fmt.Errorf("validate report schema: %w", err)
	}
	return nil
}

func loadSchema() (*jsonschema.Schema, error) {
	compileSchemaOnce.Do(func() {
		var document any
		decoder := json.NewDecoder(bytes.NewReader(schemaBytes))
		decoder.UseNumber()
		if err := decoder.Decode(&document); err != nil {
			compiledSchemaErr = fmt.Errorf("decode bundled report schema: %w", err)
			return
		}
		compiler := jsonschema.NewCompiler()
		compiler.AssertFormat()
		if err := compiler.AddResource("report-v1.schema.json", document); err != nil {
			compiledSchemaErr = fmt.Errorf("load bundled report schema: %w", err)
			return
		}
		compiledSchema, compiledSchemaErr = compiler.Compile("report-v1.schema.json")
		if compiledSchemaErr != nil {
			compiledSchemaErr = fmt.Errorf("compile bundled report schema: %w", compiledSchemaErr)
		}
	})
	return compiledSchema, compiledSchemaErr
}
