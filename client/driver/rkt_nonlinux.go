//+build !linux

package driver

import (
	"time"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// NewRktDriver returns an unimplemented driver that returns false during
// fingerprinting.
func NewRktDriver(*DriverContext) Driver {
	return RktDriver{}
}

type RktDriver struct{}

func (RktDriver) Prestart(*ExecContext, *structs.Task) (*PrestartResponse, error) {
	panic("not implemented")
}

func (RktDriver) Start(ctx *ExecContext, task *structs.Task) (*StartResponse, error) {
	panic("not implemented")
}

func (RktDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	panic("not implemented")
}

func (RktDriver) Cleanup(*ExecContext, *CreatedResources) error {
	panic("not implemented")
}

func (RktDriver) Validate(map[string]interface{}) error {
	panic("not implemented")
}

func (RktDriver) Abilities() DriverAbilities {
	panic("not implemented")
}

func (RktDriver) FSIsolation() cstructs.FSIsolation {
	panic("not implemented")
}

func (RktDriver) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	return nil
}

func (RktDriver) Periodic() (bool, time.Duration) {
	return false, 0
}
