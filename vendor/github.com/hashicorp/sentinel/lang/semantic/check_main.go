package semantic

import (
	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/token"
)

// checkMain verifies that there is a top-level `main` rule.
func checkMain(f *ast.File, fset *token.FileSet) error {
	for _, stmt := range f.Stmts {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok {
			continue
		}

		ident, ok := assign.Lhs.(*ast.Ident)
		if !ok {
			continue
		}

		// We have a main assignment!
		if ident.Name == "main" {
			return nil
		}
	}

	return &CheckError{
		Type:    CheckTypeNoMain,
		FileSet: fset,
		Message: `"main" rule was not found, every policy must have a "main" rule`,
	}
}
