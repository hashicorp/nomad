package resources

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs/config"
)

// Validator defines and interface that can be implemented to validate a value
// based on custom resources configuration.
type Validator interface {
	Type() string
	Validate(interface{}) error
}

// NewValidator is a factory method that returns concrete resource validators by type.
func NewValidator(rc *config.ResourceConfig) (Validator, error) {
	if err := rc.Validate(); err != nil {
		return nil, err
	}

	_type := rc.Config["type"]

	switch _type {
	case "range":
		return &rangeValidator{
			Upper: rc.Config["upper"].(int),
			Lower: rc.Config["lower"].(int),
		}, nil
	default:
		return nil, fmt.Errorf("error: unsuported resource type %s", _type)
	}
}

// rangeValidator is a validator that ensures resource configuration contains an integer
// value within the allowable upper and lower bounds.
type rangeValidator struct {
	Upper int
	Lower int
}

func (r *rangeValidator) Type() string {
	return "range"
}

func (r *rangeValidator) Validate(iface interface{}) error {
	val, ok := iface.(int)
	if !ok {
		return fmt.Errorf("invalid resource config: %#v cannot be cast to int64", iface)
	}

	if val < r.Lower {
		return fmt.Errorf("invalid resource config: value %d cannot be less than lower bound %d", val, r.Lower)
	}

	if val > r.Upper {
		return fmt.Errorf("invalid resource config: value %d cannot be greater than upper bound %d", val, r.Upper)
	}

	return nil
}
