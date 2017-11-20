package ast

import (
	"reflect"

	"github.com/hashicorp/sentinel/lang/token"
)

// nodeType is the pre-computed type of an ast.Node
var nodeType = reflect.TypeOf((*Node)(nil)).Elem()

// Rewrite rewrites the AST by calling the callback and replacing the
// Node called with the Node returned by the callback.
//
// This function can also be used to do a non-modifying AST walk by
// returning the node passed into the function.
//
// You can control the walking behavior by returning specialized nodes. You
// may use the RewriteSkip function to create a node that doesn't descend
// further. You may also tag fields in the AST structure with `ast:"norewrite"`
// to skip a field.
//
// This method uses reflection to recurse into the AST structure. The
// AST structures are iterated in struct field order. If an AST structure
// changes, it may change the Rewrite behavior.
func Rewrite(n Node, cb func(Node) Node) Node {
	// Perform the rewrite for this node
	n = cb(n)
	if s, ok := n.(*rewriteSkipNode); ok {
		return s.Node
	}

	// Reflect onto this node so we can recurse
	v := reflect.ValueOf(n).Elem()
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		// Check if this field is specifically tagged for no rewriting
		if t.Field(i).Tag.Get("ast") == "norewrite" {
			continue
		}

		f := v.Field(i)

		// We need to unwrap all the pointers so that we can get to the
		// inner type if an AST node is referencing an extract struct.
		fInner := f
		for fInner.Kind() == reflect.Ptr {
			fInner = reflect.Indirect(fInner)
		}

		switch fInner.Kind() {
		case reflect.Struct:
			fallthrough

		case reflect.Interface:
			if f.IsNil() {
				continue
			}

			// Interface type, verify that it is a node
			if !f.Type().Implements(nodeType) {
				continue
			}

			// Just set the node directly to the value
			f.Set(reflect.ValueOf(Rewrite(
				f.Interface().(Node), cb)))

		case reflect.Slice:
			if f.IsNil() {
				continue
			}

			// Slice, verify it is a slice of nodes
			if !f.Type().Elem().Implements(nodeType) {
				continue
			}

			// Go through each and set the value
			for i := 0; i < f.Len(); i++ {
				sliceValue := f.Index(i)
				sliceValue.Set(reflect.ValueOf(Rewrite(
					sliceValue.Interface().(Node), cb)))
			}
		}
	}

	return n
}

// RewriteSkip creates a Node that can be returned by a Rewrite
// callback to replace the node then to not descend further.
func RewriteSkip(n Node) Node {
	return &rewriteSkipNode{Node: n}
}

// rewriteSkipNode is used by RewriteSkip.
type rewriteSkipNode struct {
	Node Node
}

func (n *rewriteSkipNode) Pos() token.Pos { return 0 }
func (n *rewriteSkipNode) End() token.Pos { return 0 }
