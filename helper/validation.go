package helper

import (
	"errors"
	"fmt"

	multierror "github.com/hashicorp/go-multierror"
)

// ValidationResults stores the errors and warnings produced by validation.
//
// It should be created with NewValidationResults. Errors will be nil if there
// are no errors.
//
// It cannot be used as an error result intentionally as each use case should
// determine if Warnings should be treated as Go errors.
type ValidationResults struct {
	Errors   *multierror.Error
	Warnings []string
}

func NewValidationResults() *ValidationResults {
	return &ValidationResults{}
}

// AppendError appends an error if it is non-nil. multierror.Errors will be
// flattened.
func (v *ValidationResults) AppendError(err error) {
	if err == nil {
		return
	}
	v.Errors = multierror.Append(v.Errors, err)
}

// AppendErrorf formats the given error and then calls AppendError.
func (v *ValidationResults) AppendErrorf(format string, a ...interface{}) {
	v.AppendError(fmt.Errorf(format, a...))
}

// AppendWarning appends a warning if it is not empty.
func (v *ValidationResults) AppendWarning(warning string) {
	if warning == "" {
		return
	}
	v.Warnings = append(v.Warnings, warning)
}

// AppendWarningf formats the given string and then calls AppendWarning.
func (v *ValidationResults) AppendWarningf(format string, a ...interface{}) {
	v.AppendWarning(fmt.Sprintf(format, a...))
}

// AppendWarnings appends all non-empty warnings.
func (v *ValidationResults) AppendWarnings(warnings ...string) {
	for _, w := range warnings {
		v.AppendWarning(w)
	}
}

// Err returns nil if there are no errors, otherwise it returns the value of
// Errors.
func (v *ValidationResults) Err() error {
	return v.Errors.ErrorOrNil()
}

// ErrSummary returns nil if there are no errors, otherwise it returns an error
// containing a summary of the warnings and errors.
func (v *ValidationResults) ErrSummary() error {
	if v.Errors == nil {
		return nil
	}

	ne := len(v.Errors.Errors)

	if ne == 0 {
		return nil
	}

	var errstr string

	switch ne {
	case 1:
		errstr = "1 error"
	default:
		errstr = fmt.Sprintf("%d errors", ne)
	}

	switch nw := len(v.Warnings); nw {
	case 1:
		errstr += ", 1 warning"
	default:
		errstr += fmt.Sprintf(", %d warnings", nw)
	}

	return errors.New(errstr)
}

// Problems returns true if there are any errors or warnings.
func (v *ValidationResults) Problems() bool {
	if v.Errors == nil {
		return len(v.Warnings) > 0
	}

	return len(v.Errors.Errors) > 0 || len(v.Warnings) > 0
}
