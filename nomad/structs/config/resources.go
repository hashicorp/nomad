package config

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
)

// ResourceConfig is used to configure a custom resource explicitly
type ResourceConfig struct {
	Name   string                 `hcl:",key"`
	Config map[string]interface{} `hcl:"config"`
	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

func (r *ResourceConfig) Validate() error {
	_type := r.Config["type"]
	if _type == "" {
		return fmt.Errorf("error: resource %s requires a type config entry", r.Name)
	}

	switch _type {
	case "range":
		return validateRangeConfig(r)
	default:
		return fmt.Errorf("error: resource %s has unsupported type %s", r.Name, _type)
	}
}

func validateRangeConfig(r *ResourceConfig) error {
	mErr := new(multierror.Error)

	lower, ok := r.Config["lower"]
	if !ok {
		mErr = multierror.Append(mErr, fmt.Errorf("error: resource %s of type range has no lower bound", r.Name))
	}

	var lowerVal, upperVal int
	if lower != nil {
		lowerVal, ok = lower.(int)
		if !ok {
			mErr = multierror.Append(mErr, fmt.Errorf("error: resource %s of type range has lower bound %#v which cannot be cast to int", r.Name, lower))
		}
	}

	upper, ok := r.Config["upper"]
	if !ok {
		mErr = multierror.Append(mErr, fmt.Errorf("error: resource %s of type range has no upper bound", r.Name))
	}

	if upper != nil {
		upperVal, ok = upper.(int)
		if !ok {
			mErr = multierror.Append(mErr, fmt.Errorf("error: resource %s of type range has lower bound %#v which cannot be cast to int", r.Name, lower))
		}
	}

	if lowerVal > upperVal {
		mErr = multierror.Append(mErr, fmt.Errorf("error: resource %s of type range has lower bound %d which is greater than upper bound %d", r.Name, lowerVal, upperVal))
	}

	return mErr.ErrorOrNil()
}
