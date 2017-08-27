package eval

import (
	"github.com/hashicorp/sentinel/lang/ast"
)

// rewriteImportSelector is an ast rewrite function that detects selector
// expressions that are import accesses and rewrites them to use astImportExpr.
func rewriteImportSelector() func(ast.Node) ast.Node {
	// Predeclare the function so that we can recursively call it within itself
	var f func(n ast.Node) ast.Node
	f = func(n ast.Node) ast.Node {
		// If this is a call expression then determine if it is an import call.
		if cx, ok := n.(*ast.CallExpr); ok {
			// Rewrite the function expression. If it turns into an import
			// expression then this is an import function call. We only have
			// to modify the import expression slightly to note it is a call.
			if ix, ok := f(cx.Fun).(*astImportExpr); ok {
				ix.EndPos = cx.End() // EndPos is now the end of the call
				ix.Args = cx.Args    // Populate args
				return ix            // Return the import expression itself
			}
		}

		// Looking for a selector
		sx, ok := n.(*ast.SelectorExpr)
		if !ok {
			return n
		}

		// Track the parents up the root so we can get all the keys.
		// Optimistically track keys since most selector expressions are
		// imports.
		root := sx
		keys := []string{sx.Sel.Name}
		for {
			parent, ok := root.X.(*ast.SelectorExpr)
			if !ok {
				break
			}

			root = parent
			keys = append(keys, parent.Sel.Name)
		}

		// The root expression must be an identifier
		ident, ok := root.X.(*ast.Ident)
		if !ok {
			return n
		}

		// Reverse the keys so they're in the right order
		for i := len(keys)/2 - 1; i >= 0; i-- {
			opp := len(keys) - 1 - i
			keys[i], keys[opp] = keys[opp], keys[i]
		}

		// It appears to be an import access.
		return &astImportExpr{
			Original: sx,
			StartPos: sx.Pos(),
			EndPos:   sx.End(),
			Import:   ident.Name,
			Keys:     keys,
		}
	}

	return f
}
