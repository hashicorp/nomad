// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package addrs

// InputVariable is the address of an input variable.
type InputVariable struct {
	referenceable
	Name string
}

func (v InputVariable) String() string {
	return "var." + v.Name
}
