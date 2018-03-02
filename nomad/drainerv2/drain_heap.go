package drainerv2

import (
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

type DrainDeadlineNotifier interface {
	NextBatch() <-chan []*structs.Node
	Remove(nodeID string)
	Watch(nodeID string, deadline time.Time)
}

type deadlineHeap struct {
}

func (d *deadlineHeap) NextBatch() <-chan []structs.Node        { return nil }
func (d *deadlineHeap) Remove(nodeID string)                    {}
func (d *deadlineHeap) Watch(nodeID string, deadline time.Time) {}
