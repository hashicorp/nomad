package format

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/mitchellh/colorstring"
	"github.com/hashicorp/sentinel/lang/object"
	"github.com/hashicorp/sentinel/lang/printer"
	"github.com/hashicorp/sentinel/lang/token"
	"github.com/hashicorp/sentinel/runtime/trace"
)

// RuleTrace is a formatter for rule traces.
type RuleTrace struct {
	FileSet *token.FileSet         // FileSet for accurate line numbers
	Rules   map[string]*trace.Rule // Ruule map from trace.Trace
	Color   *colorstring.Colorize  // If non-nil, color the output with this
}

// String returns the format for this set of rule traces. This cannot be
// called concurrently.
func (f *RuleTrace) String() string {
	// If no color is specified, then create a disabled colorizer
	if f.Color == nil {
		f.Color = &colorstring.Colorize{
			Colors:  colorstring.DefaultColors,
			Disable: true,
		}
	}

	// We'll collect output here
	var buf bytes.Buffer

	// Output the main rule first if there is one
	if r, ok := f.Rules["main"]; ok {
		f.stringRule(&buf, r)
	}

	// Do other rules, first sort by their name
	keys := make([]string, 0, len(f.Rules))
	for k, _ := range f.Rules {
		if k != "main" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	// Output each rule
	for _, k := range keys {
		buf.WriteRune('\n')
		f.stringRule(&buf, f.Rules[k])
	}

	return buf.String()
}

func (f *RuleTrace) stringRule(buf *bytes.Buffer, r *trace.Rule) {
	if r == nil {
		// Shouldn't ever happen, but if it does just ignore it.
		return
	}

	buf.WriteString(f.Color.Color(fmt.Sprintf(
		"[reset][bold][%s]%s - %s - Rule %q\n",
		f.valueColor(r.Root.Value),
		f.valueText(r.Root.Value),
		f.FileSet.Position(r.Pos),
		r.Ident)))

	for _, c := range r.Root.Children {
		f.stringBool(buf, c, 1)
	}

}

func (f *RuleTrace) stringBool(buf *bytes.Buffer, b *trace.Bool, indent int) {
	var fmtOut bytes.Buffer
	if err := printer.Fprint(&fmtOut, f.FileSet, b.Expr); err != nil {
		fmtOut.WriteString(fmt.Sprintf("error printing: %s", err))
	}

	prefix := strings.Repeat(" ", indent*2)
	buf.WriteString(f.Color.Color(fmt.Sprintf(
		"[reset]%s[%s]%s - %s - %s\n",
		prefix,
		f.valueColor(b.Value),
		f.valueText(b.Value),
		f.FileSet.Position(b.Expr.Pos()),
		fmtOut.String())))

	for _, c := range b.Children {
		f.stringBool(buf, c, indent+1)
	}

	// If we're at the final leaf of an undefined value in a trace,
	// then we show where that undefined value was created.
	if len(b.Children) == 0 && b.Value.Type() == object.UNDEFINED {
		undef := b.Value.(*object.UndefinedObj)
		buf.WriteString(f.Color.Color(fmt.Sprintf(
			"[reset]%s  [%s]Undefined originated at: %s\n",
			prefix,
			f.valueColor(b.Value),
			f.FileSet.Position(undef.Pos[0]))))
	}
}

func (f *RuleTrace) valueColor(v object.Object) string {
	switch v {
	case object.True:
		return "green"

	case object.False:
		return "red"

	default:
		return "yellow"
	}
}

func (f *RuleTrace) valueText(v object.Object) string {
	switch v {
	case nil:
		// A nil value can only mean that an error occurred at this point.
		return "ERROR"

	case object.True:
		return "TRUE"

	case object.False:
		return "FALSE"

	default:
		// If it is undefined, we show a special value
		if v.Type() == object.UNDEFINED {
			return "UNDEF"
		}

		return fmt.Sprintf("OTHER: %s", strings.ToUpper(v.Type().String()))
	}
}
