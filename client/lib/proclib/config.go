package proclib

import (
	"github.com/hashicorp/go-hclog"
)

// Configs is used to pass along values from client configuration that are
// build-tag specific. These are not the final representative values, just what
// was set in agent configuration.
type Configs struct {
}

func (c *Configs) Log(log hclog.Logger) {
	log.Trace(
		"configs",
		// "parent_cgroup", c.ParentCgroup,
		// "reserved_cores", c.ReservedCores,
		// "total_compute", c.TotalCompute,
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
