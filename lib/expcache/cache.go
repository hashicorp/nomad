// Package expcache provides an expiring cache implementation.
package expcache

import "sync"

type Cache[V any] struct {
	f GetFunc[V]

	lock  sync.Mutex
	items map[string]V // pair w/ time.Time (YOU ARE HERE)
}

// GetFunc implements the way in which a value is retrieved if it is
// not found in the cache, or if a value in the cache has expired.
type GetFunc[V any] func(string) (V, error)

func New[V any](getter GetFunc[V]) *Cache[V] {
	return &Cache[V]{
		items: make(map[string]V),
	}
}

func (c *Cache[V]) Get(key string) (V, error) {
	// todo: cache
	return c.f(key)
}
