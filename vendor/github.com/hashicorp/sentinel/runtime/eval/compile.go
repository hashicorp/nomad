package eval

import (
	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/semantic"
	"github.com/hashicorp/sentinel/lang/token"
)

// Compiled is a "compiled" policy. This is a processed policy that
// contains various AST modifications and preprocessing and is ready
// for execution.
//
// A compiled policy can be evaluated concurrently.
type Compiled struct {
	file    *ast.File      // File to execute
	fileSet *token.FileSet // FileSet for positional information
}

// CompileOpts are options for compilation.
type CompileOpts struct {
	File    *ast.File      // File to execute
	FileSet *token.FileSet // FileSet for positional information
}

// Compile compiles the given policy file.
//
// Because evaluation is done via an interpreter, "compile" means to rewrite
// the AST in some forms and to precompute some values. It results in a
// more efficient execution for the interpreter.
//
// Once a policy has been compiled, the AST must not be reused. It will be
// modified in-place.
func Compile(opts *CompileOpts) (*Compiled, error) {
	// Verify semantics
	if err := semantic.Check(opts.File, opts.FileSet); err != nil {
		return nil, err
	}

	// Rewrite the import expressions
	file := ast.Rewrite(opts.File, rewriteImportSelector()).(*ast.File)

	// Build
	return &Compiled{
		file:    file,
		fileSet: opts.FileSet,
	}, nil
}
