package fingerprint

import (
	"github.com/hashicorp/nomad/client/lib/cgutil"
)

func (f *CPUFingerprint) deriveReservableCores(req *FingerprintRequest) ([]uint16, error) {
	// The cpuset cgroup manager is initialized (on linux), but not accessible
	// from the finger-printer. So we reach in and grab the information manually.
	// We may assume the hierarchy is already setup.
	return cgutil.GetCPUsFromCgroup(req.Config.CgroupParent)
}
