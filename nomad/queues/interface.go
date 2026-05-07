// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import "github.com/hashicorp/nomad/nomad/structs"

type Queue interface {
	Enqueue(*structs.Evaluation)
}
