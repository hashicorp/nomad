package resources

import (
	"fmt"
)

// ResourceType is a discriminator that identifies the type of resource being loaded
// and defines the dynamic behaviors that each implementation is expected to provide.
type ResourceType interface {
	Name() string
	Validate(interface{}) error
}

// Range is a ResourceType that ensures resource configuration contains an integer
// value within the allowable upper and lower bounds.
type Range struct {
	Upper int64 `hcl:"upper"`
	Lower int64 `hcl:"lower"`
}

func (r *Range) Name() string {
	return "range"
}

func (r *Range) Validate(config interface{}) error {
	val, ok := config.(int64)
	if !ok {
		return fmt.Errorf("invalid resource config: %#v cannot be cast to int64", config)
	}

	if val < r.Lower {
		return fmt.Errorf("invalid resource config: %d cannot be less than lower bound %d", val, r.Lower)
	}

	if val > r.Upper {
		return fmt.Errorf("invalid resource config: %d cannot be greater than upper bound %d", val, r.Upper)
	}

	return nil
}

// Resource is a custom resource that users can configure to expose custom capabilities
// available per client.
type Resource struct {
	Name  string `hcl:"name,key"`
	Range Range  `mapstructure:"range" hcl:"range,block,optional"`
}
