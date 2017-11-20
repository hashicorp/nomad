package eval

import (
	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/token"
)

// Custom AST nodes that the interpreter uses to make certain actions faster.

// astImportExpr is an ast.Node that represents a potential import field access
// and optionally a function call. It is potentially an import access and not
// definitely because the Import field may be shadowed by a variable
// assignment. In this case, the Original expression should be evaluated.
type astImportExpr struct {
	Original         ast.Node   `ast:"norewrite"` // original in case import is shadowed
	StartPos, EndPos token.Pos  // position of the selector and end
	Import           string     // Import being accessed
	Keys             []string   // Keys being accessed
	Args             []ast.Expr // Arguments (nil if not a call expression)
}

func (n *astImportExpr) Pos() token.Pos { return n.StartPos }
func (n *astImportExpr) End() token.Pos { return n.EndPos }
func (n *astImportExpr) ExprNode()      {}

// printer.Printable
func (n *astImportExpr) PrintNode() ast.Node { return n.Original }
