// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package getter

// Error is a RecoverableError used to include the URL along with the underlying
// fetching error.
type Error struct {
	URL         string
	Err         error
	Recoverable bool
}

func (e *Error) Error() string {
	if e == nil || e.Err == nil {
		return "<nil>"
	}
	return e.Err.Error()
}

func (e *Error) IsRecoverable() bool {
	return e.Recoverable
}

func (e *Error) Equal(o *Error) bool {
	if e == nil || o == nil {
		return e == o
	}

	switch {
	case e.URL != o.URL:
		return false
	case e.Recoverable != o.Recoverable:
		return false
	case e.Error() != o.Error():
		return false
	default:
		return true
	}
}
