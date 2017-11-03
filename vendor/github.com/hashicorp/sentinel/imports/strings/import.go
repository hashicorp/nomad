// Package strings contains a Sentinel plugin for performing string operations.
package strings

import (
	"strings"

	"github.com/hashicorp/sentinel-sdk"
	"github.com/hashicorp/sentinel-sdk/framework"
)

// New creates a new Import.
func New() sdk.Import {
	return &framework.Import{
		Root: &root{},
	}
}

type root struct{}

// framework.Root impl.
func (m *root) Configure(raw map[string]interface{}) error {
	return nil
}

// framework.Namespace impl.
func (m *root) Get(key string) (interface{}, error) {
	return nil, nil
}

// framework.Call impl.
func (m *root) Func(key string) interface{} {
	switch key {
	case "has_prefix":
		return func(s, prefix string) (interface{}, error) {
			return strings.HasPrefix(s, prefix), nil
		}

	case "has_suffix":
		return func(s, suffix string) (interface{}, error) {
			return strings.HasSuffix(s, suffix), nil
		}

	case "trim_prefix":
		return func(s, prefix string) (interface{}, error) {
			return strings.TrimPrefix(s, prefix), nil
		}

	case "trim_suffix":
		return func(s, suffix string) (interface{}, error) {
			return strings.TrimSuffix(s, suffix), nil
		}

	case "to_lower":
		return func(s string) (interface{}, error) {
			return strings.ToLower(s), nil
		}

	case "to_upper":
		return func(s string) (interface{}, error) {
			return strings.ToUpper(s), nil
		}
	case "split":
		return func(in interface{}, sep string) (interface{}, error) {
			switch in.(type) {
			case string:
				return strings.Split(in.(string), sep), nil

			// As a special case, if the string is already split, return as-is.
			// This makes it safe to call split functions when not knowing if
			// the value is already split or not.
			case []string:
				return in.([]string), nil
			case []interface{}:
				// JSON decoding sometimes produces this for string lists
				return in, nil
			}
			return nil, nil
		}
	}

	return nil
}
