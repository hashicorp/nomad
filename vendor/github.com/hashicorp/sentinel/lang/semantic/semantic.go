// Package semantic contains runtime-agnostic semantic checks on a parsed
// AST. This verifies semantic behavior such as `main` existing, functions
// having a return statement, and more.
//
// All semantic checks made here are according to the language specification.
// For runtime-specific behavior, a runtime may want to add their own checks.
package semantic

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/token"
)

// Check performs semantic checks on the given file.
//
// The returned error may be multiple errors contained within a go-multierror
// structure. You can type assert on the result to extract individual errors.
func Check(f *ast.File, fset *token.FileSet) error {
	var err *multierror.Error
	if e := checkMain(f, fset); e != nil {
		err = multierror.Append(err, e)
	}
	return err.ErrorOrNil()
}

// CheckError is the error type that is returned for semantic check failures.
type CheckError struct {
	Type    CheckType // Unique string for the type of error
	FileSet *token.FileSet
	Pos     token.Pos
	Message string
}

// CheckType is an enum of the various types of check errors that can exist.
// A single check type may correspond to different error messages but
// represents a broad category of similar errors.
type CheckType string

const (
	CheckTypeNoMain CheckType = "no-main"
)

func (e *CheckError) Error() string {
	var buf bytes.Buffer

	if e.Pos.IsValid() {
		buf.WriteString(fmt.Sprintf("%s: ", e.FileSet.Position(e.Pos)))
	}

	buf.WriteString(e.Message)
	return buf.String()
}
