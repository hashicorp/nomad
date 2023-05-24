// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

func (m *Meta) NodePoolPredictor(filter *set.Set[string]) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := m.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.NodePools, nil)
		if err != nil {
			return nil
		}

		results := resp.Matches[contexts.NodePools]
		if filter == nil {
			return results
		}

		filtered := []string{}
		for _, pool := range resp.Matches[contexts.NodePools] {
			if filter.Contains(pool) {
				continue
			}
			filtered = append(filtered, pool)
		}

		return filtered
	})
}
