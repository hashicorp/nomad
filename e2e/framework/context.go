// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package framework

import (
	"testing"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F is the framework context that is passed to each test.
// It is used to access the *testing.T context as well as testify helpers
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
type F struct {
	id string
	*require.Assertions
	assert *assert.Assertions
	t      *testing.T

	data map[interface{}]interface{}
}

func newF(t *testing.T) *F {
	return newFWithID(uuid.Generate()[:8], t)
}

func newFFromParent(f *F, t *testing.T) *F {
	child := newF(t)
	for k, v := range f.data {
		child.Set(k, v)
	}
	return child
}

func newFWithID(id string, t *testing.T) *F {
	ft := &F{
		id:         id,
		t:          t,
		Assertions: require.New(t),
		assert:     assert.New(t),
	}

	return ft
}

// Assert fetches an assert flavor of testify assertions
// https://godoc.org/github.com/stretchr/testify/assert
//
// Deprecated: no longer use e2e/framework for new tests; see TestExample for new e2e test structure.
func (f *F) Assert() *assert.Assertions {
	return f.assert
}

// T returns the *testing.T context
func (f *F) T() *testing.T {
	return f.t
}

// ID returns the current context ID
func (f *F) ID() string {
	return f.id
}

// Set is used to set arbitrary key/values to pass between before/after and test methods
func (f *F) Set(key, val interface{}) {
	f.data[key] = val
}

// Value retrives values set by the F.Set method
func (f *F) Value(key interface{}) interface{} {
	return f.data[key]
}
