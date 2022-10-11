package loader

import (
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/logging"
)

var (
	// AgentSupportedApiVersions is the set of API versions supported by the
	// Nomad agent by plugin type.
	AgentSupportedApiVersions = map[string][]string{
		base.PluginTypeDevice:  {device.ApiVersion010},
		base.PluginTypeDriver:  {drivers.ApiVersion010},
		base.PluginTypeLogging: {logging.ApiVersion010},
	}
)
