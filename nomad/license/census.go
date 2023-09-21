// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package license

import (
	census "github.com/hashicorp/go-census/schema"
)

// Creates a new census schema used by census agent
// to export mentioned metrics
func NewCensusSchema() census.Schema {
	return census.Schema{
		Version: "1.0.0",
		Service: "nomad",
		Metrics: []census.Metric{
			{
				Key:         "nomad.billable.clients",
				Description: "Number of client nodes in the cluster",
				Kind:        "counter",
				Mode:        "write",
			},
		},
	}
}
