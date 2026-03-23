package check

import "fmt"

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
