// +build pro ent

package structs

import (
	"fmt"
	"regexp"

	multierror "github.com/hashicorp/go-multierror"
)

var (
	// validNamespaceName is used to validate a namespace name
	validNamespaceName = regexp.MustCompile("^[a-zA-Z0-9-]{1,128}$")
)

const (
	// maxNamespaceDescriptionLength limits a namespace description length
	maxNamespaceDescriptionLength = 256
)

// Namespace allows logically grouping jobs and their associated objects.
type Namespace struct {
	// Name is the name of the namespace
	Name string

	// Description is a human readable description of the namespace
	Description string

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64
}

func (n *Namespace) Validate() error {
	var mErr multierror.Error

	// Validate the name and description
	if !validNamespaceName.MatchString(n.Name) {
		err := fmt.Errorf("invalid name %q. Must match regex %s", n.Name, validNamespaceName)
		mErr.Errors = append(mErr.Errors, err)
	}
	if len(n.Description) > maxNamespaceDescriptionLength {
		err := fmt.Errorf("description longer than %d", maxNamespaceDescriptionLength)
		mErr.Errors = append(mErr.Errors, err)
	}

	return mErr.ErrorOrNil()
}

func (n *Namespace) Copy() *Namespace {
	nc := new(Namespace)
	*nc = *n
	return nc
}
