// +build !linux

package driver

import cstructs "github.com/hashicorp/nomad/client/structs"

func (d *JavaDriver) FSIsolation() cstructs.FSIsolation {
	return cstructs.FSIsolationNone
}
