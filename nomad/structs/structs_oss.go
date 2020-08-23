// +build !ent

package structs

import "errors"

func (m *Multiregion) Validate(jobType string, jobDatacenters []string) error {
	if m != nil {
		return errors.New("Multiregion jobs are unlicensed.")
	}

	return nil
}
