// +build pro ent

package mock

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

func Namespace() *structs.Namespace {
	return &structs.Namespace{
		Name:        fmt.Sprintf("team-%s", structs.GenerateUUID()),
		Description: "test namespace",
		CreateIndex: 100,
		ModifyIndex: 200,
	}
}
