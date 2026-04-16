package queues

import "github.com/hashicorp/nomad/nomad/structs"

type Queue interface {
	Enqueue(*structs.Evaluation)
}
