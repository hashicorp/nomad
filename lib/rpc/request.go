package rpc

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

type Request interface {
	structs.RPCInfo
	RequestToken() string
	RequestNamespace() string
	SetRegion(string)
	SetIdentity(*structs.AuthenticatedIdentity)
	GetIdentity() *structs.AuthenticatedIdentity
}

type Response any
