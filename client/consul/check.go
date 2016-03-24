package consul

import (
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
)

type Check interface {
	Run() *cstructs.CheckResult
	ID() string
}
