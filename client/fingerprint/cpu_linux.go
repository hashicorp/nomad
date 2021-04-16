package fingerprint

import (
	"github.com/hashicorp/nomad/client/lib/cgutil"
)

func (f *CPUFingerprint) deriveReservableCores(req *FingerprintRequest) ([]uint16, error) {
	parent := req.Config.CgroupParent
	if parent == "" {
		parent = cgutil.DefaultCgroupParent
	}
	return cgutil.GetCPUsFromCgroup(parent)
}
