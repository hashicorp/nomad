// +build pro ent

package mock

import (
	"fmt"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

func Namespace() *structs.Namespace {
	ns := &structs.Namespace{
		Name:        fmt.Sprintf("team-%s", uuid.Generate()),
		Description: "test namespace",
		CreateIndex: 100,
		ModifyIndex: 200,
	}
	ns.SetHash()
	return ns
}
