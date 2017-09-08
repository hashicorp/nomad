package static

type mapNS struct {
	// objects is the mapping of available attributes for this data.
	objects map[string]interface{}
}

// framework.Namespace impl.
func (m *mapNS) Get(key string) (interface{}, error) {
	result, ok := m.objects[key]
	if !ok {
		return nil, nil
	}

	return reflectReturn(result), nil
}

// framework.Map impl.
func (m *mapNS) Map() (map[string]interface{}, error) {
	return m.objects, nil
}
