package printer

import (
	"github.com/hashicorp/sentinel/lang/ast"
)

// Printable is an optional interface that an ast.Node can implement
// to specify a printable format for the node. This is only valid for
// custom ast.Node implementations.
type Printable interface {
	// PrintNode returns the ast.Node to use instead of this one for printing.
	PrintNode() ast.Node
}
