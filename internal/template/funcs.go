package template

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

func fromJSON(s string) []map[string]any {
	var result []map[string]any
	_ = json.Unmarshal([]byte(s), &result)
	return result
}

func toJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// safeGet returns the value at a dot-separated path from a nested structure.
// Signature: safeGet(path string, data any) any
//
// Supported:
//   - Struct fields by exact field name (case-sensitive)
//   - Map lookups by string key
//   - Slice/array elements by numeric index in the path
//   - Transparent unwrapping of pointers and interfaces at each step
//
// Behavior:
//   - If any step is nil, missing, or out of range, it returns nil.
//   - Final value is returned as interface{}; non-nil pointers/interfaces are unwrapped.
//
// Examples (for templates):
//
//	{{ safeGet "User.Name" . }}
//	{{ safeGet "Items.0.Title" . }}
//
// Limitations:
//   - Does NOT read struct tags (e.g., `json:"name"`)
//   - No case-insensitive field matching
//   - Map keys are not converted (expects string keys)
func safeGet(path string, data any) any {
	parts := strings.Split(path, ".")
	val := reflect.ValueOf(data)

	for _, p := range parts {
		for val.IsValid() && (val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface) {
			if val.IsNil() {
				return nil
			}
			val = val.Elem()
		}
		if !val.IsValid() {
			return nil
		}

		switch val.Kind() {
		case reflect.Struct:
			fv := val.FieldByName(p)
			if !fv.IsValid() {
				return nil
			}
			val = fv

		case reflect.Map:
			mv := val.MapIndex(reflect.ValueOf(p))
			if !mv.IsValid() {
				return nil
			}
			val = mv

		case reflect.Slice, reflect.Array:
			idx, err := strconv.Atoi(p)
			if err != nil || idx < 0 || idx >= val.Len() {
				return nil
			}
			val = val.Index(idx)

		default:
			return nil
		}
	}

	for val.IsValid() && (val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface) {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	if !val.IsValid() {
		return nil
	}
	return val.Interface()
}

// safeGetOr is like safeGet but returns a default value if result is nil.
// Example: safeGetOr("User.Name", data, "Anonymous")
func safeGetOr(path string, data any, def any) any {
	v := safeGet(path, data)
	if v == nil {
		return def
	}
	return v
}
