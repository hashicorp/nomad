package fingerprint

import (
	"github.com/hashicorp/nomad/client/lib/cgutil"
)

func (f *CPUFingerprint) deriveReservableCores(req *FingerprintRequest, totalCores int) ([]uint16, error) {
	if req.Config.DisableCgroupManagement {
		return defaultReservableCores(totalCores), nil
	}
	return cgutil.GetCPUsFromCgroup(req.Config.CgroupParent)

}
