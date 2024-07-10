// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import "reflect"

type CNIArgs struct {
	Args map[string]string
}

func (d *CNIArgs) Copy() *CNIArgs {
	if d == nil {
		return nil
	}
	newMap := make(map[string]string)
	for k, v := range d.Args {
		newMap[k] = v
	}
	return &CNIArgs{
		Args: newMap,
	}
}

func (d *CNIArgs) Equal(o *CNIArgs) bool {
	if d == nil || o == nil {
		return d == o
	}
	return reflect.DeepEqual(d.Args, o.Args)
}
