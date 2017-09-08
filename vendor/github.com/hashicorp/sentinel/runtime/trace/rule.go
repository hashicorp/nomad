package trace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/object"
	"github.com/hashicorp/sentinel/lang/printer"
	"github.com/hashicorp/sentinel/lang/token"
)

// Rule is a trace of a top-level rule expression.
//
// Rules are evaluated only once, per the specification. A trace explains
// the evaluation of a rule based on the various boolean expressions evaluated
// within it.
type Rule struct {
	Ident string    // Identifier of the rule
	Pos   token.Pos // Position of the rule ident assignment
	Root  *Bool     // Root boolean expression
}

// Bool is a trace of a boolean expression.
type Bool struct {
	Expr     ast.Expr      // Expr is the expression that was executed
	Value    object.Object // Resulting value (true, false, undefined)
	Children []*Bool       // Child expressions, in left-to-right order
}

// String output for a Bool is the rendered source.
func (b *Bool) String() string {
	var fmtOut bytes.Buffer
	if err := printer.Fprint(&fmtOut, token.NewFileSet(), b.Expr); err != nil {
		fmtOut.Reset()
		fmtOut.WriteString(fmt.Sprintf("error formatting: %s", err))
	}

	return fmtOut.String()
}

// encoding/json implementation to allow embedding in JSON.
func (b *Bool) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"expression": b.String(),
		"value":      b.Value.String(),
		"children":   b.Children,
	})
}

// String outputs a human-friendly string format representing the trace
// of the rule. This format is very rudimentary and only really meant for
// debugging.
func (r *Rule) String() string {
	var fmt ruleFormatter
	return fmt.Format(r)
}

// encoding/json implementation to allow embedding in JSON.
func (r *Rule) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"ident":  r.Ident,
		"string": r.String(),
		"root":   r.Root,
	})
}

type ruleFormatter struct {
	indentLevel int
	buf         bytes.Buffer
}

func (f *ruleFormatter) Format(r *Rule) string {
	f.buf.Reset()
	f.indentLevel = 0

	f.output(fmt.Sprintf(
		"Rule %q (byte offset %d) = %s", r.Ident, r.Pos, r.Root.Value))
	for _, c := range r.Root.Children {
		f.formatBool(c)
	}

	return f.buf.String()
}

func (f *ruleFormatter) formatBool(b *Bool) {
	f.indent()
	defer f.unindent()

	var fmtOut bytes.Buffer
	if err := printer.Fprint(&fmtOut, token.NewFileSet(), b.Expr); err != nil {
		fmtOut.Reset()
		fmtOut.WriteString(fmt.Sprintf("error formatting: %s", err))
	}

	f.output(fmt.Sprintf("%s (offset %d): %s", b.Value, b.Expr.Pos(), fmtOut.String()))
	for _, c := range b.Children {
		f.formatBool(c)
	}
}

func (f *ruleFormatter) output(msg string) {
	f.buf.WriteString(strings.Repeat(" ", f.indentLevel*2))
	f.buf.WriteString(msg + "\n")
}

func (f *ruleFormatter) indent() {
	f.indentLevel++
}

func (f *ruleFormatter) unindent() {
	f.indentLevel--
}
