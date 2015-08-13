package scheduler

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestRandomIterator(t *testing.T) {
	ctx := NewEvalContext()
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		nodes = append(nodes, mock.Node())
	}

	nc := make([]*structs.Node, len(nodes))
	copy(nc, nodes)
	rand := NewRandomIterator(ctx, nc)

	var out []*structs.Node
	for {
		next := rand.Next()
		if next == nil {
			break
		}
		out = append(out, next)
	}

	if len(out) != len(nodes) {
		t.Fatalf("missing nodes")
	}
	if reflect.DeepEqual(out, nodes) {
		t.Fatalf("same order")
	}
}

func TestDriverIterator(t *testing.T) {
	ctx := NewEvalContext()
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	static := NewStaticIterator(ctx, nodes)

	nodes[0].Attributes["driver.foo"] = "2"
	nodes[2].Attributes["driver.foo"] = "2"

	drivers := map[string]struct{}{
		"docker": struct{}{},
		"foo":    struct{}{},
	}
	driver := NewDriverIterator(ctx, static, drivers)

	var out []*structs.Node
	for {
		next := driver.Next()
		if next == nil {
			break
		}
		out = append(out, next)
	}

	if len(out) != 2 {
		t.Fatalf("missing nodes")
	}
	if out[0] != nodes[0] || out[1] != nodes[2] {
		t.Fatalf("bad: %#v", out)
	}
}
