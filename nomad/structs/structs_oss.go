//go:build !ent
// +build !ent

package structs

import (
	"errors"
	"fmt"

	multierror "github.com/hashicorp/go-multierror"
)

func (m *Multiregion) Validate(jobType string, jobDatacenters []string) error {
	if m != nil {
		return errors.New("Multiregion jobs are unlicensed.")
	}

	return nil
}

func (p *ScalingPolicy) validateType() multierror.Error {
	var mErr multierror.Error

	// Check policy type and target
	switch p.Type {
	case ScalingPolicyTypeHorizontal:
		targetErr := p.validateTargetHorizontal()
		mErr.Errors = append(mErr.Errors, targetErr.Errors...)
	default:
		mErr.Errors = append(mErr.Errors, fmt.Errorf(`scaling policy type "%s" is not valid`, p.Type))
	}

	return mErr
}

func (j *Job) GetEntScalingPolicies() []*ScalingPolicy {
	return nil
}
