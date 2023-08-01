package jobspec

// flattenMapSlice flattens any occurrences of []map[string]interface{} into
// map[string]interface{}.
func flattenMapSlice(m map[string]interface{}) map[string]interface{} {
	newM := make(map[string]interface{}, len(m))

	for k, v := range m {
		var newV interface{}

		switch mapV := v.(type) {
		case []map[string]interface{}:
			// Recurse into each map and flatten values
			newMap := map[string]interface{}{}
			for _, innerM := range mapV {
				for innerK, innerV := range flattenMapSlice(innerM) {
					newMap[innerK] = innerV
				}
			}
			newV = newMap

		case map[string]interface{}:
			// Recursively flatten maps
			newV = flattenMapSlice(mapV)

		default:
			newV = v
		}

		newM[k] = newV
	}

	return newM
}
