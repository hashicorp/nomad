package framework

// MapFromKeys creates a map[string]interface{} for a Namespace from the
// given set of keys. This is a useful helper for implementing the Map
// interface.
func MapFromKeys(ns Namespace, keys []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, k := range keys {
		var err error
		result[k], err = ns.Get(k)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}
