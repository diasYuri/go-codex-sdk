package codex

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
)

var tomlBareKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func serializeConfigOverrides(configOverrides CodexConfigObject) ([]string, error) {
	var overrides []string
	if err := flattenConfigOverrides(configOverrides, "", &overrides); err != nil {
		return nil, err
	}
	return overrides, nil
}

func flattenConfigOverrides(value CodexConfigValue, prefix string, overrides *[]string) error {
	object, ok := asStringMap(value)
	if !ok {
		if prefix == "" {
			return errorsNew("codex config overrides must be a plain object")
		}
		rendered, err := toTomlValue(value, prefix)
		if err != nil {
			return err
		}
		*overrides = append(*overrides, prefix+"="+rendered)
		return nil
	}

	keys := sortedKeys(object)
	if prefix == "" && len(keys) == 0 {
		return nil
	}
	if prefix != "" && len(keys) == 0 {
		*overrides = append(*overrides, prefix+"={}")
		return nil
	}

	for _, key := range keys {
		if key == "" {
			return errorsNew("codex config override keys must be non-empty strings")
		}
		child := object[key]
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if childObject, ok := asStringMap(child); ok {
			if err := flattenConfigOverrides(childObject, path, overrides); err != nil {
				return err
			}
			continue
		}
		rendered, err := toTomlValue(child, path)
		if err != nil {
			return err
		}
		*overrides = append(*overrides, path+"="+rendered)
	}

	return nil
}

func toTomlValue(value CodexConfigValue, path string) (string, error) {
	if value == nil {
		return "", fmt.Errorf("codex config override at %s cannot be nil", path)
	}
	if object, ok := asStringMap(value); ok {
		return inlineTomlObject(object, path)
	}

	switch typed := value.(type) {
	case string:
		return quoteString(typed), nil
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	case int:
		return strconv.Itoa(typed), nil
	case int8:
		return strconv.FormatInt(int64(typed), 10), nil
	case int16:
		return strconv.FormatInt(int64(typed), 10), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint64:
		return strconv.FormatUint(typed, 10), nil
	case float32:
		return formatFloat(float64(typed), path, 32)
	case float64:
		return formatFloat(typed, path, 64)
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		parts := make([]string, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			rendered, err := toTomlValue(rv.Index(i).Interface(), fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return "", err
			}
			parts = append(parts, rendered)
		}
		return "[" + joinComma(parts) + "]", nil
	}

	return "", fmt.Errorf("unsupported codex config override value at %s: %T", path, value)
}

func formatFloat(value float64, path string, bitSize int) (string, error) {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return "", fmt.Errorf("codex config override at %s must be a finite number", path)
	}
	return strconv.FormatFloat(value, 'f', -1, bitSize), nil
}

func inlineTomlObject(object CodexConfigObject, path string) (string, error) {
	keys := sortedKeys(object)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			return "", errorsNew("codex config override keys must be non-empty strings")
		}
		rendered, err := toTomlValue(object[key], path+"."+key)
		if err != nil {
			return "", err
		}
		parts = append(parts, formatTomlKey(key)+" = "+rendered)
	}
	return "{" + joinComma(parts) + "}", nil
}

func asStringMap(value CodexConfigValue) (CodexConfigObject, bool) {
	switch typed := value.(type) {
	case CodexConfigObject:
		return typed, true
	case map[string]CodexConfigValue:
		return CodexConfigObject(typed), true
	case map[string]any:
		return mapAnyToConfig(typed), true
	}

	rv := reflect.ValueOf(value)
	if !rv.IsValid() || rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	out := make(CodexConfigObject, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		out[iter.Key().String()] = iter.Value().Interface()
	}
	return out, true
}

func mapAnyToConfig(input map[string]any) CodexConfigObject {
	out := make(CodexConfigObject, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneConfigObject(input CodexConfigObject) CodexConfigObject {
	if input == nil {
		return nil
	}
	out := make(CodexConfigObject, len(input))
	for key, value := range input {
		if object, ok := asStringMap(value); ok {
			out[key] = cloneConfigObject(object)
			continue
		}
		out[key] = value
	}
	return out
}

func sortedKeys(input CodexConfigObject) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatTomlKey(key string) string {
	if tomlBareKey.MatchString(key) {
		return key
	}
	return quoteString(key)
}

func quoteString(value string) string {
	rendered, _ := json.Marshal(value)
	return string(rendered)
}

func joinComma(parts []string) string {
	out := ""
	for i, part := range parts {
		if i > 0 {
			out += ", "
		}
		out += part
	}
	return out
}

func errorsNew(message string) error {
	return fmt.Errorf("%s", message)
}
