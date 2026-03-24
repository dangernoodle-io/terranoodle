package fields

import "fmt"

// ExtractStrings converts a map[string]interface{} (from terraform-json
// Change.After) to map[string]string, keeping only string, bool, and
// float64 values. For float64, it formats whole numbers without a decimal point.
func ExtractStrings(after interface{}) map[string]string {
	fields := make(map[string]string)
	m, ok := after.(map[string]interface{})
	if !ok {
		return fields
	}
	for k, v := range m {
		switch val := v.(type) {
		case string:
			fields[k] = val
		case bool:
			fields[k] = fmt.Sprintf("%v", val)
		case float64:
			if val == float64(int64(val)) {
				fields[k] = fmt.Sprintf("%d", int64(val))
			} else {
				fields[k] = fmt.Sprintf("%g", val)
			}
		}
	}
	return fields
}
