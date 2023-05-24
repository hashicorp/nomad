// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
)

func (m *Meta) formatNodePoolList(pools []*api.NodePool) string {
	out := make([]string, len(pools)+1)
	out[0] = "Name|Description"
	for i, p := range pools {
		out[i+1] = fmt.Sprintf("%s|%s",
			p.Name,
			p.Description,
		)
	}
	return formatList(out)
}
