package check

import (
	"encoding/json"
	"fmt"

	"github.com/legostin/constitution/pkg/types"
)

// toStringSlice converts an interface{} (from YAML) to []string.
func toStringSlice(v interface{}) ([]string, error) {
	switch val := v.(type) {
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("expected string, got %T", item)
			}
			result = append(result, s)
		}
		return result, nil
	case []string:
		return val, nil
	default:
		return nil, fmt.Errorf("expected slice, got %T", v)
	}
}

// toSliceOfMaps converts an interface{} (from YAML) to []map[string]interface{}.
func toSliceOfMaps(v interface{}) ([]map[string]interface{}, error) {
	switch val := v.(type) {
	case []interface{}:
		result := make([]map[string]interface{}, 0, len(val))
		for _, item := range val {
			m, ok := item.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("expected map, got %T", item)
			}
			result = append(result, m)
		}
		return result, nil
	case []map[string]interface{}:
		return val, nil
	default:
		return nil, fmt.Errorf("expected slice of maps, got %T", v)
	}
}

// extractContent extracts the scannable content from a HookInput.
// For Edit tool it returns new_string, for others it returns the specified scanField.
func extractContent(input *types.HookInput, scanField string) (string, error) {
	if input.ToolInput == nil {
		return "", nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(input.ToolInput, &m); err != nil {
		return "", err
	}
	if input.ToolName == "Edit" {
		if ns, ok := m["new_string"].(string); ok {
			return ns, nil
		}
		return "", nil
	}
	val, ok := m[scanField]
	if !ok {
		return "", nil
	}
	str, ok := val.(string)
	if !ok {
		return "", nil
	}
	return str, nil
}
