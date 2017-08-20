package plugins

import (
	"fmt"

	driver "github.com/hashicorp/nomad/client/drivernew/plugin"
)

// Serve is used to start a plugin's RPC server. It takes an interface that must
// implement a known plugin interface to Nomad.
func Serve(plugin interface{}) {
	switch p := plugin.(type) {
	case driver.Driver:
		driver.Serve(p)
	default:
		fmt.Println("Unsupported plugin type")
	}
}
