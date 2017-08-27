package object

// Scope maintains the set of declared entities and a link to the outer scope.
// This is the main structure used for identifier lookup.
type Scope struct {
	Outer   *Scope            // Parent scope
	Objects map[string]Object // Objects in this scope by identifier
}

// NewScope creates a new scope nested in the outer scope.
func NewScope(outer *Scope) *Scope {
	const n = 4 // initial scope capacity
	return &Scope{Outer: outer, Objects: make(map[string]Object, n)}
}

// Scope returns the Scope for the given key. If key doesn't exist, it
// returns s. This allows assigning values to the proper scope.
func (s *Scope) Scope(key string) *Scope {
	top := s
	for s != nil {
		if _, ok := s.Objects[key]; ok {
			return s
		}

		s = s.Outer
	}

	return top
}

// Lookup performs a lookup for an identifier with the given name. This will
// automatically look in outer scopes.
func (s *Scope) Lookup(key string) Object {
	for s != nil {
		v, ok := s.Objects[key]
		if ok {
			return v
		}

		s = s.Outer
	}

	return nil
}
