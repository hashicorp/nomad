// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"maps"
)

type CNIConfig struct {
	Args map[string]string
}

func (d *CNIConfig) Copy() *CNIConfig {
	if d == nil {
		return nil
	}
	newMap := make(map[string]string)
	for k, v := range d.Args {
		newMap[k] = v
	}
	return &CNIConfig{
		Args: newMap,
	}
}

func (d *CNIConfig) Equal(o *CNIConfig) bool {
	if d == nil || o == nil {
		return d == o
	}
	return maps.Equal(d.Args, o.Args)
}
