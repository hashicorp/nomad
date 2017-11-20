package sentinel

import (
	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/parser"
	"github.com/hashicorp/sentinel/lang/semantic"
	"github.com/hashicorp/sentinel/lang/token"
)

// Compile parses and verifies the given input policy. The returned values
// can be used with policy.SetPolicy.
//
// "name" is a filename for the policy and is used for errors. src can be
// an io.Reader, []byte, or string and will be used to read the source.
//
// This is safe to be called concurrently.
func (s *Sentinel) Compile(name string, src interface{}) (*ast.File, *token.FileSet, error) {
	// Parse
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, nil, err
	}

	// Perform semantic checks
	if err := semantic.Check(f, fset); err != nil {
		return nil, nil, err
	}

	// Return the reuslt
	return f, fset, nil
}
