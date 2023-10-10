// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import "slices"

type Action struct {
	Name    string
	Command string
	Args    []string
}

func (a *Action) Copy() *Action {
	if a == nil {
		return nil
	}
	na := new(Action)
	*na = *a
	na.Args = slices.Clone(a.Args)
	return na
}

func (a *Action) Equal(o *Action) bool {
	if a == o {
		return true
	}
	if a == nil || o == nil {
		return false
	}
	return a.Name == o.Name &&
		a.Command == o.Command &&
		slices.Equal(a.Args, o.Args)
}
