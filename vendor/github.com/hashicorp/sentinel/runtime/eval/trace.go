package eval

import (
	"bytes"

	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/object"
	"github.com/hashicorp/sentinel/runtime/trace"
)

//-------------------------------------------------------------------
// Rule tracing

func (e *evalState) pushTrace(n ast.Expr) *trace.Bool {
	if e.Trace == nil {
		return nil
	}

	b := &trace.Bool{
		Expr: n,
	}

	e.traceStack = append(e.traceStack, b)

	return b
}

func (e *evalState) popTrace(v object.Object) object.Object {
	if e.Trace == nil {
		return v
	}

	idx := len(e.traceStack) - 1

	// Set the current value
	current := e.traceStack[idx]
	current.Value = v

	// If we have a parent, then we set ourselves
	if idx > 0 {
		parent := e.traceStack[idx-1]
		parent.Children = append(parent.Children, current)
	}

	// Unwind
	e.traceStack[idx] = nil
	e.traceStack = e.traceStack[:idx]

	return v
}

//-------------------------------------------------------------------
// Printing

func (e *evalState) funcPrint(args []object.Object) (interface{}, error) {
	// This function only does something if we're tracing.
	if e.Trace != nil {
		e.funcPrintBuf(&e.Trace.Print, args)
		e.Trace.Print.WriteRune('\n')
	}

	return nil, nil
}

func (e *evalState) funcPrintBuf(buf *bytes.Buffer, args []object.Object) {
	for i, arg := range args {
		switch x := arg.(type) {
		case *object.StringObj:
			buf.WriteString(x.Value)

		default:
			buf.WriteString(arg.String())
		}

		if i < len(args)-1 {
			buf.WriteRune(' ')
		}
	}
}
