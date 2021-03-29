package fingerprint

import (
	"github.com/hashicorp/nomad/client/lib/cgutil"
)

func (f *CPUFingerprint) deriveReservableCores(req *FingerprintRequest) ([]uint16, error) {
	return cgutil.GetCPUsFromCgroup(req.Config.CgroupParent)
}
