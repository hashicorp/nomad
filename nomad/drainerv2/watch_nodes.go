package drainerv2

import "github.com/hashicorp/nomad/nomad/structs"

type DrainingNodeWatcher interface {
	Transistioning() <-chan []*structs.Node
}
