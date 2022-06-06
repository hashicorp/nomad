package resources

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
)

// Resource is a custom resource that users can configure to expose custom capabilities
// available per client.
type Resource struct {
	Name  string `hcl:"key"`
	Range *Range `hcl:"range,expand,optional"`
}

func (r *Resource) ValidateConfig() error {
	mErr := new(multierror.Error)

	if r.Range != nil {
		if err := r.Range.validateConfig(); err != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("invalid config: resource %s of type range returned error - %s", r.Name, err.Error()))
		}
	}

	return mErr.ErrorOrNil()
}

// Range is a ResourceType that ensures resource configuration contains an integer
// value within the allowable upper and lower bounds.
type Range struct {
	Upper int `hcl:"upper"`
	Lower int `hcl:"lower"`
}

func (r *Range) Validate(val int) error {
	if val < r.Lower {
		return fmt.Errorf("invalid resource config: value %d cannot be less than lower bound %d", val, r.Lower)
	}

	if val > r.Upper {
		return fmt.Errorf("invalid resource config: value %d cannot be greater than upper bound %d", val, r.Upper)
	}

	return nil
}

func (r *Range) validateConfig() error {
	mErr := new(multierror.Error)

	if r.Lower > r.Upper {
		mErr = multierror.Append(mErr, fmt.Errorf("lower bound %d is greater than upper bound %d", r.Lower, r.Upper))
	}

	return mErr.ErrorOrNil()
}
