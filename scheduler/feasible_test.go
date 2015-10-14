package scheduler

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestStaticIterator_Reset(t *testing.T) {
	_, ctx := testContext(t)
	var nodes []*structs.Node
	for i := 0; i < 3; i++ {
		nodes = append(nodes, mock.Node())
	}
	static := NewStaticIterator(ctx, nodes)

	for i := 0; i < 6; i++ {
		static.Reset()
		for j := 0; j < i; j++ {
			static.Next()
		}
		static.Reset()

		out := collectFeasible(static)
		if len(out) != len(nodes) {
			t.Fatalf("out: %#v", out)
			t.Fatalf("missing nodes %d %#v", i, static)
		}

		ids := make(map[string]struct{})
		for _, o := range out {
			if _, ok := ids[o.ID]; ok {
				t.Fatalf("duplicate")
			}
			ids[o.ID] = struct{}{}
		}
	}
}

func TestStaticIterator_SetNodes(t *testing.T) {
	_, ctx := testContext(t)
	var nodes []*structs.Node
	for i := 0; i < 3; i++ {
		nodes = append(nodes, mock.Node())
	}
	static := NewStaticIterator(ctx, nodes)

	newNodes := []*structs.Node{mock.Node()}
	static.SetNodes(newNodes)

	out := collectFeasible(static)
	if !reflect.DeepEqual(out, newNodes) {
		t.Fatalf("bad: %#v", out)
	}
}

func TestRandomIterator(t *testing.T) {
	_, ctx := testContext(t)
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		nodes = append(nodes, mock.Node())
	}

	nc := make([]*structs.Node, len(nodes))
	copy(nc, nodes)
	rand := NewRandomIterator(ctx, nc)

	out := collectFeasible(rand)
	if len(out) != len(nodes) {
		t.Fatalf("missing nodes")
	}
	if reflect.DeepEqual(out, nodes) {
		t.Fatalf("same order")
	}
}

func TestDriverIterator(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	static := NewStaticIterator(ctx, nodes)

	nodes[0].Attributes["driver.foo"] = "1"
	nodes[2].Attributes["driver.foo"] = "1"

	drivers := map[string]struct{}{
		"exec": struct{}{},
		"foo":  struct{}{},
	}
	driver := NewDriverIterator(ctx, static, drivers)

	out := collectFeasible(driver)
	if len(out) != 2 {
		t.Fatalf("missing nodes")
	}
	if out[0] != nodes[0] || out[1] != nodes[2] {
		t.Fatalf("bad: %#v", out)
	}
}

func TestConstraintIterator(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	static := NewStaticIterator(ctx, nodes)

	nodes[0].Attributes["kernel.name"] = "freebsd"
	nodes[1].Datacenter = "dc2"

	constraints := []*structs.Constraint{
		&structs.Constraint{
			Hard:    true,
			Operand: "=",
			LTarget: "$node.datacenter",
			RTarget: "dc1",
		},
		&structs.Constraint{
			Hard:    true,
			Operand: "is",
			LTarget: "$attr.kernel.name",
			RTarget: "linux",
		},
	}
	constr := NewConstraintIterator(ctx, static, constraints)

	out := collectFeasible(constr)
	if len(out) != 1 {
		t.Fatalf("missing nodes")
	}
	if out[0] != nodes[2] {
		t.Fatalf("bad: %#v", out)
	}
}

func TestResolveConstraintTarget(t *testing.T) {
	type tcase struct {
		target string
		node   *structs.Node
		val    interface{}
		result bool
	}
	node := mock.Node()
	cases := []tcase{
		{
			target: "$node.id",
			node:   node,
			val:    node.ID,
			result: true,
		},
		{
			target: "$node.datacenter",
			node:   node,
			val:    node.Datacenter,
			result: true,
		},
		{
			target: "$node.name",
			node:   node,
			val:    node.Name,
			result: true,
		},
		{
			target: "$node.foo",
			node:   node,
			result: false,
		},
		{
			target: "$attr.kernel.name",
			node:   node,
			val:    node.Attributes["kernel.name"],
			result: true,
		},
		{
			target: "$attr.rand",
			node:   node,
			result: false,
		},
		{
			target: "$meta.pci-dss",
			node:   node,
			val:    node.Meta["pci-dss"],
			result: true,
		},
		{
			target: "$meta.rand",
			node:   node,
			result: false,
		},
	}

	for _, tc := range cases {
		res, ok := resolveConstraintTarget(tc.target, tc.node)
		if ok != tc.result {
			t.Fatalf("TC: %#v, Result: %v %v", tc, res, ok)
		}
		if ok && !reflect.DeepEqual(res, tc.val) {
			t.Fatalf("TC: %#v, Result: %v %v", tc, res, ok)
		}
	}
}

func TestCheckConstraint(t *testing.T) {
	type tcase struct {
		op         string
		lVal, rVal interface{}
		result     bool
	}
	cases := []tcase{
		{
			op:   "=",
			lVal: "foo", rVal: "foo",
			result: true,
		},
		{
			op:   "is",
			lVal: "foo", rVal: "foo",
			result: true,
		},
		{
			op:   "==",
			lVal: "foo", rVal: "foo",
			result: true,
		},
		{
			op:   "!=",
			lVal: "foo", rVal: "foo",
			result: false,
		},
		{
			op:   "!=",
			lVal: "foo", rVal: "bar",
			result: true,
		},
		{
			op:   "not",
			lVal: "foo", rVal: "bar",
			result: true,
		},
		{
			op:   "version",
			lVal: "1.2.3", rVal: "~> 1.0",
			result: true,
		},
		{
			op:   "regexp",
			lVal: "foobarbaz", rVal: "[\\w]+",
			result: true,
		},
		{
			op:   "<",
			lVal: "foo", rVal: "bar",
			result: false,
		},
	}

	for _, tc := range cases {
		_, ctx := testContext(t)
		if res := checkConstraint(ctx, tc.op, tc.lVal, tc.rVal); res != tc.result {
			t.Fatalf("TC: %#v, Result: %v", tc, res)
		}
	}
}

func TestCheckLexicalOrder(t *testing.T) {
	type tcase struct {
		op         string
		lVal, rVal interface{}
		result     bool
	}
	cases := []tcase{
		{
			op:   "<",
			lVal: "bar", rVal: "foo",
			result: true,
		},
		{
			op:   "<=",
			lVal: "foo", rVal: "foo",
			result: true,
		},
		{
			op:   ">",
			lVal: "bar", rVal: "foo",
			result: false,
		},
		{
			op:   ">=",
			lVal: "bar", rVal: "bar",
			result: true,
		},
		{
			op:   ">",
			lVal: 1, rVal: "foo",
			result: false,
		},
	}
	for _, tc := range cases {
		if res := checkLexicalOrder(tc.op, tc.lVal, tc.rVal); res != tc.result {
			t.Fatalf("TC: %#v, Result: %v", tc, res)
		}
	}
}

func TestCheckVersionConstraint(t *testing.T) {
	type tcase struct {
		lVal, rVal interface{}
		result     bool
	}
	cases := []tcase{
		{
			lVal: "1.2.3", rVal: "~> 1.0",
			result: true,
		},
		{
			lVal: "1.2.3", rVal: ">= 1.0, < 1.4",
			result: true,
		},
		{
			lVal: "2.0.1", rVal: "~> 1.0",
			result: false,
		},
		{
			lVal: "1.4", rVal: ">= 1.0, < 1.4",
			result: false,
		},
		{
			lVal: 1, rVal: "~> 1.0",
			result: true,
		},
	}
	for _, tc := range cases {
		_, ctx := testContext(t)
		if res := checkVersionConstraint(ctx, tc.lVal, tc.rVal); res != tc.result {
			t.Fatalf("TC: %#v, Result: %v", tc, res)
		}
	}
}

func TestCheckRegexpConstraint(t *testing.T) {
	type tcase struct {
		lVal, rVal interface{}
		result     bool
	}
	cases := []tcase{
		{
			lVal: "foobar", rVal: "bar",
			result: true,
		},
		{
			lVal: "foobar", rVal: "^foo",
			result: true,
		},
		{
			lVal: "foobar", rVal: "^bar",
			result: false,
		},
		{
			lVal: "zipzap", rVal: "foo",
			result: false,
		},
		{
			lVal: 1, rVal: "foo",
			result: false,
		},
	}
	for _, tc := range cases {
		_, ctx := testContext(t)
		if res := checkRegexpConstraint(ctx, tc.lVal, tc.rVal); res != tc.result {
			t.Fatalf("TC: %#v, Result: %v", tc, res)
		}
	}
}

func collectFeasible(iter FeasibleIterator) (out []*structs.Node) {
	for {
		next := iter.Next()
		if next == nil {
			break
		}
		out = append(out, next)
	}
	return
}
