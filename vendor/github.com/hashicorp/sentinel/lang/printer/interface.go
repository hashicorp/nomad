package printer

import (
	"io"
	"text/tabwriter"

	"github.com/hashicorp/sentinel/lang/token"
)

// Fprint prints the given value using the printer to the specified output.
// This uses the default configuration.
func Fprint(output io.Writer, fset *token.FileSet, node interface{}) error {
	p := newPrinter()
	p.fset = fset
	if err := p.printNode(node); err != nil {
		return err
	}

	// redirect output through a trimmer to eliminate trailing whitespace
	// (Input to a tabwriter must be untrimmed since trailing tabs provide
	// formatting information. The tabwriter could provide trimming
	// functionality but no tabwriter is used when RawFormat is set.)
	output = &trimmer{output: output}

	// Use a tabwriter to properly align columns of text
	twmode := tabwriter.DiscardEmptyColumns | tabwriter.TabIndent
	output = tabwriter.NewWriter(output, 0, 8, 1, ' ', twmode)
	if _, err := output.Write(p.output); err != nil {
		return err
	}

	// flush tabwriter, if any
	if tw, _ := output.(*tabwriter.Writer); tw != nil {
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	return nil
}
