// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package state

import (
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

var EnterpriseMsgTypeEvents = map[structs.MessageType]string{}

func enterpriseEventFromChangeDeleted(_ memdb.Change) (structs.Event, bool) {
	return structs.Event{}, false
}

func enterpriseEventFromChange(_ memdb.Change) (structs.Event, bool) {
	return structs.Event{}, false
}
