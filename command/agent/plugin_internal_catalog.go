package agent

import (

	// Each internal plugin has an init func which registers itself with the
	// plugin catalog. Since the plugin implementations are not imported by the
	// client or server they must be referenced here so plugin registration
	// occures.

	// raw_exec driver
	_ "github.com/hashicorp/nomad/drivers/rawexec"
)
