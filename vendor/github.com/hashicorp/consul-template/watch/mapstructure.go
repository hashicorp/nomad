package watch

import (
	"reflect"

	"github.com/mitchellh/mapstructure"
)

// StringToWaitDurationHookFunc returns a function that converts strings to wait
// value. This is designed to be used with mapstructure for parsing out a wait
// value.
func StringToWaitDurationHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}
		if t != reflect.TypeOf(new(Wait)) {
			return data, nil
		}

		// Convert it by parsing
		return ParseWait(data.(string))
	}
}
