//+build darwin dragonfly freebsd netbsd openbsd solaris windows

package driver

import (
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
)

func (d *ExecDriver) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	d.fingerprintSuccess = helper.BoolToPtr(false)
	resp.Detected = true
	return nil
}
