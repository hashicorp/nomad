// Package time contains a Sentinel plugin for serving time information.
//
// Docs will come soon, a big TODO. For now please see the source below
// which isn't too difficult to follow.
package time

import (
	"fmt"
	"time"

	"github.com/hashicorp/sentinel-sdk"
	"github.com/hashicorp/sentinel-sdk/framework"
	"github.com/mitchellh/mapstructure"
)

// New creates a new Import.
func New() sdk.Import {
	return &framework.Import{
		Root: &root{},
	}
}

type config struct {
	Timezone  string
	FixedTime int64 `mapstructure:"fixed_time"`
}

type root struct {
	conf config

	location *time.Location
}

// framework.Root impl.
func (m *root) Configure(raw map[string]interface{}) error {
	if err := mapstructure.WeakDecode(raw, &m.conf); err != nil {
		return err
	}

	if v := m.conf.Timezone; v != "" {
		loc, err := time.LoadLocation(v)
		if err != nil {
			return fmt.Errorf(
				"Error loading default location %q: %s", v, err)
		}

		m.location = loc
	}

	return nil
}

// framework.NamespaceCreator impl.
//
// The time package creates a namespace for each execution so that the
// time is constant as it is executed throughout the policy.
func (m *root) Namespace() framework.Namespace {
	var refTime time.Time
	if v := m.conf.FixedTime; v > 0 {
		// We have a fixed time, so use that directly
		refTime = time.Unix(v, 0).UTC()
	} else {
		// We use local time. If a timezone is specified, we use the
		// specified timezone as the default.
		refTime = time.Now()
		if m.location != nil {
			refTime = refTime.In(m.location)
		}
	}

	return &rootNamespace{
		refTime: refTime,
	}
}

// rootNamespace is the root-level namespace.
type rootNamespace struct {
	refTime time.Time
}

// framework.Namespace impl.
func (m *rootNamespace) Get(key string) (interface{}, error) {
	switch key {
	case "day":
		return m.refTime.Day(), nil

	case "hour":
		return m.refTime.Hour(), nil

	case "minute":
		return m.refTime.Minute(), nil

	case "month":
		return m.refTime.Month(), nil

	case "second":
		return m.refTime.Second(), nil

	case "weekday":
		return m.refTime.Weekday(), nil

	case "year":
		return m.refTime.Year(), nil

	default:
		return nil, fmt.Errorf("unsupported field: %s", key)
	}
}

// framework.Map impl.
func (m *rootNamespace) Map() (map[string]interface{}, error) {
	// This is not ideal, I'm open to doing this another way.
	keys := []string{"day", "hour", "minute", "month", "second", "weekday", "year"}

	// Go through all the supported keys and build our map data.
	result := make(map[string]interface{})
	for _, k := range keys {
		v, err := m.Get(k)
		if err != nil {
			return nil, err
		}

		result[k] = v
	}

	return result, nil
}
