package proclib

import (
	"github.com/hashicorp/go-hclog"
)

// Configs is used to pass along values from client configuration that are
// build-tag specific. These are not the final representative values, just what
// was set in agent configuration.
type Configs struct {
	// ParentCgroup can be set in Nomad client config. By default this value
	// is "/nomad" on cgroups v1, and "nomad.slice" in cgroups v2.
	//
	// Linux only.
	ParentCgroup string

	// ReservedCores can be set in Nomad client config. By default this value is
	// not set, implying there are no reserved cores (i.e. the Nomad client may
	// use all cores on the system).
	ReservedCores []uint16

	// TotalCompute can be set in Nomad client config. By default this value is
	// not set, implying the the client should automatically detect the total
	// available compute by scanning the system.
	TotalCompute int
}

func (c *Configs) Log(log hclog.Logger) {
	log.Trace(
		"configs",
		"parent_cgroup", c.ParentCgroup,
		"reserved_cores", c.ReservedCores,
		"total_compute", c.TotalCompute,
	)
}

// YOU ARE HERE
// who will answer questions about the final values of the above?
//
// parentCgroup - wrangler implementation
//
// reserved_cores -
//  cconfig reserved_cores
//  cconfig reserved.cores
//  cconfig reserved.cpu
//
// total_compute -
//
