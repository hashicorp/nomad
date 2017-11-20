// Package time contains a Sentinel plugin for serving time information.
//
// Docs will come soon, a big TODO. For now please see the source below
// which isn't too difficult to follow.
package time

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/sentinel-sdk"
	"github.com/hashicorp/sentinel-sdk/framework"
	"github.com/mitchellh/mapstructure"
)

type rawTime struct {
	Reference int64 `mapstructure:"reference"`
}

func parseTimeInput(in interface{}) (time.Time, error) {
	if in == nil {
		return time.Time{}, fmt.Errorf("nil input to time parser")
	}

	jsonIn, ok := in.(json.Number)
	if ok {
		var err error
		in, err = jsonIn.Int64()
		if err != nil {
			return time.Time{}, err
		}
	}

	switch in.(type) {
	case string:
		val, err := time.Parse(time.RFC3339Nano, in.(string))
		if err != nil {
			return time.Time{}, err
		}
		return val, nil

	case time.Time:
		return in.(time.Time), nil

	case int, int32, int64, uint, uint32, uint64, float32, float64:
		var raw rawTime
		if err := mapstructure.WeakDecode(map[string]interface{}{"reference": in}, &raw); err != nil {
			return time.Time{}, err
		}
		return time.Unix(raw.Reference, 0).UTC(), nil

	case map[string]interface{}:
		raw, ok := in.(map[string]interface{})["rfc3339"]
		if !ok {
			return time.Time{}, fmt.Errorf("could not find current time")
		}
		rawStr, ok := raw.(string)
		if !ok {
			return time.Time{}, fmt.Errorf("incorrect time format for current time")
		}
		val, err := time.Parse(time.RFC3339Nano, rawStr)
		if err != nil {
			return time.Time{}, err
		}
		return val, nil
	}

	return time.Time{}, fmt.Errorf("unknown time format")
}

// Input must be in nanoseconds, such as output by the time.hour, time.second,
// etc. The exception to this is suffixed strings, which are more
// human-friendly
func parseDurationInput(in interface{}) (time.Duration, error) {
	var dur time.Duration
	jsonIn, ok := in.(json.Number)
	if ok {
		in = jsonIn.String()
	}

	switch in.(type) {
	case string:
		inp := in.(string)
		if inp == "" {
			return time.Duration(0), nil
		}
		var err error
		// Look for a suffix otherwise its a plain second value
		if strings.HasSuffix(inp, "s") || strings.HasSuffix(inp, "m") || strings.HasSuffix(inp, "h") {
			dur, err = time.ParseDuration(inp)
			if err != nil {
				return dur, err
			}
		} else {
			// Plain integer
			secs, err := strconv.ParseInt(inp, 10, 64)
			if err != nil {
				return dur, err
			}
			dur = time.Duration(secs) * time.Nanosecond
		}
	case int:
		dur = time.Duration(in.(int)) * time.Nanosecond
	case int32:
		dur = time.Duration(in.(int32)) * time.Nanosecond
	case int64:
		dur = time.Duration(in.(int64)) * time.Nanosecond
	case uint:
		dur = time.Duration(in.(uint)) * time.Nanosecond
	case uint32:
		dur = time.Duration(in.(uint32)) * time.Nanosecond
	case uint64:
		dur = time.Duration(in.(uint64)) * time.Nanosecond
	default:
		return 0, errors.New("could not parse duration from input")
	}

	return dur, nil
}

// New creates a new Import.
func New() sdk.Import {
	return &framework.Import{
		Root: &root{},
	}
}

type root struct {
	reference time.Time
	location  *time.Location
}

// framework.Root impl.
func (m *root) Configure(raw map[string]interface{}) error {
	var err error
	if v, ok := raw["reference"]; ok {
		m.reference, err = parseTimeInput(v)
		if err != nil {
			return err
		}
	}

	if v, ok := raw["timezone"]; ok {
		if tzStr, ok := v.(string); ok && tzStr != "" {
			loc, err := time.LoadLocation(tzStr)
			if err != nil {
				return fmt.Errorf(
					"Error loading default location %q: %s", tzStr, err)
			}
			m.location = loc
		}
	}

	return nil
}

// framework.NamespaceCreator impl.
//
// The time package creates a namespace for each execution so that the
// time is constant as it is executed throughout the policy.
func (m *root) Namespace() framework.Namespace {
	t := m.reference
	if t.IsZero() {
		t = time.Now().UTC()
	}
	if m.location != nil {
		t = t.In(m.location)
	}
	return &rootNamespace{
		reference: t,
	}
}

// rootNamespace is the root-level namespace.
type rootNamespace struct {
	reference time.Time
}

// framework.Namespace impl.
func (m *rootNamespace) Get(key string) (interface{}, error) {
	switch key {
	case "now":
		return &fixedTimespace{fixedTime: m.reference}, nil

	case "nanosecond":
		return time.Nanosecond, nil

	case "microsecond":
		return time.Microsecond, nil

	case "millisecond":
		return time.Millisecond, nil

	case "second":
		return time.Second, nil

	case "minute":
		return time.Minute, nil

	case "hour":
		return time.Hour, nil

	default:
		return nil, fmt.Errorf("unsupported field: %s", key)
	}
}

// framework.Call impl.
func (r *rootNamespace) Func(key string) interface{} {
	switch key {
	case "load":
		return func(in interface{}) (interface{}, error) {
			val, err := parseTimeInput(in)
			if err != nil {
				return nil, err
			}
			return &fixedTimespace{fixedTime: val}, nil
		}
	}
	return nil
}

// Holds a fixed time that is not the ref time. Created via the `now` or `load`
// functions.
type fixedTimespace struct {
	fixedTime time.Time
}

// framework.Namespace impl.
func (f *fixedTimespace) Get(key string) (interface{}, error) {
	switch key {
	case "day":
		return f.fixedTime.Day(), nil

	case "hour":
		return f.fixedTime.Hour(), nil

	case "minute":
		return f.fixedTime.Minute(), nil

	case "month":
		return f.fixedTime.Month(), nil

	case "second":
		return f.fixedTime.Second(), nil

	case "weekday":
		return f.fixedTime.Weekday(), nil

	case "year":
		return f.fixedTime.Year(), nil

	case "unix":
		return f.fixedTime.Unix(), nil

	case "unix_nano":
		return f.fixedTime.UnixNano(), nil

	default:
		return nil, fmt.Errorf("unsupported field: %s", key)
	}
}

// framework.Call impl.
func (f *fixedTimespace) Func(key string) interface{} {
	switch key {
	case "before":
		return func(in interface{}) (interface{}, error) {
			val, err := parseTimeInput(in)
			if err != nil {
				return nil, err
			}
			return f.fixedTime.Before(val), nil
		}
	case "after":
		return func(in interface{}) (interface{}, error) {
			val, err := parseTimeInput(in)
			if err != nil {
				return nil, err
			}
			return f.fixedTime.After(val), nil
		}
	case "equal":
		return func(in interface{}) (interface{}, error) {
			val, err := parseTimeInput(in)
			if err != nil {
				return nil, err
			}
			return f.fixedTime.Equal(val), nil
		}
	case "add":
		return func(in interface{}) (interface{}, error) {
			dur, err := parseDurationInput(in)
			if err != nil {
				return nil, err
			}
			return &fixedTimespace{fixedTime: f.fixedTime.Add(dur)}, nil
		}
	case "sub":
		return func(in interface{}) (interface{}, error) {
			val, err := parseTimeInput(in)
			if err != nil {
				return nil, err
			}
			return f.fixedTime.Sub(val), nil
		}
	}
	return nil
}

// framework.Map impl.
func (f *fixedTimespace) Map() (map[string]interface{}, error) {
	// This is not ideal, I'm open to doing this another way.
	keys := []string{"day", "hour", "minute", "month", "second", "weekday", "year", "unix", "unix_nano"}

	// Go through all the supported keys and build our map data.
	result := make(map[string]interface{})
	for _, k := range keys {
		v, err := f.Get(k)
		if err != nil {
			return nil, err
		}

		result[k] = v
	}

	// Access to itself must be formatted as a string or gRPC throws errors
	// encoding
	result["rfc3339"] = f.fixedTime.Format(time.RFC3339Nano)

	return result, nil
}
