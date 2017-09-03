// +build pro ent

package structs

import (
	"fmt"
	"regexp"

	multierror "github.com/hashicorp/go-multierror"
)

// Offset the Nomad Pro specific values so that we don't overlap
// the OSS/Enterprise values.
const (
	NamespaceUpsertRequestType MessageType = (64 + iota)
	NamespaceDeleteRequestType
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

// NamespaceListRequest is used to request a list of namespaces
type NamespaceListRequest struct {
	QueryOptions
}

// NamespaceListResponse is used for a list request
type NamespaceListResponse struct {
	Namespaces []*Namespace
	QueryMeta
}

// NamespaceSpecificRequest is used to query a specific namespace
type NamespaceSpecificRequest struct {
	Name string
	QueryOptions
}

// SingleNamespaceResponse is used to return a single namespace
type SingleNamespaceResponse struct {
	Namespace *Namespace
	QueryMeta
}

// NamespaceDeleteRequest is used to delete a namespace
type NamespaceDeleteRequest struct {
	Name string
	WriteRequest
}

// NamespaceUpsertRequest is used to upsert a namespace
type NamespaceUpsertRequest struct {
	Namespace *Namespace
	WriteRequest
}
