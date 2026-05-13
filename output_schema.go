package codex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
)

type outputSchemaFile struct {
	schemaPath string
	cleanup    func()
}

func createOutputSchemaFile(turnOptions *TurnOptions) (outputSchemaFile, error) {
	if turnOptions == nil || turnOptions.OutputSchema == nil {
		return outputSchemaFile{cleanup: func() {}}, nil
	}
	if !isJSONObject(turnOptions.OutputSchema) {
		return outputSchemaFile{}, errors.New("outputSchema must be a plain JSON object")
	}

	schemaDir, err := os.MkdirTemp("", "codex-output-schema-")
	if err != nil {
		return outputSchemaFile{}, err
	}
	cleanup := func() {
		_ = os.RemoveAll(schemaDir)
	}

	schemaPath := filepath.Join(schemaDir, "schema.json")
	data, err := json.Marshal(turnOptions.OutputSchema)
	if err != nil {
		cleanup()
		return outputSchemaFile{}, err
	}
	if err := os.WriteFile(schemaPath, data, 0o600); err != nil {
		cleanup()
		return outputSchemaFile{}, err
	}

	return outputSchemaFile{schemaPath: schemaPath, cleanup: cleanup}, nil
}

func isJSONObject(value any) bool {
	if value == nil {
		return false
	}
	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return false
		}
		rv = rv.Elem()
	}
	return rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String
}
