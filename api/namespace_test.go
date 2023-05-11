// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestNamespaces_Register(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	namespaces := c.Namespaces()

	// Create a namespace and register it
	ns := testNamespace()
	wm, err := namespaces.Register(ns, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the jobs back out again
	resp, qm, err := namespaces.List(nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 2, resp)
	must.Eq(t, ns.Name, resp[0].Name)
	must.Eq(t, "default", resp[1].Name)
}

func TestNamespaces_Register_Invalid(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	namespaces := c.Namespaces()

	// Create an invalid namespace and register it
	ns := testNamespace()
	ns.Name = "*"
	_, err := namespaces.Register(ns, nil)
	must.ErrorContains(t, err, `invalid name "*".`)
}

func TestNamespaces_Info(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	namespaces := c.Namespaces()

	// Trying to retrieve a namespace before it exists returns an error
	_, _, err := namespaces.Info("foo", nil)
	must.NotNil(t, err)
	must.ErrorContains(t, err, "not found")

	// Register the namespace
	ns := testNamespace()
	wm, err := namespaces.Register(ns, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the namespace again and ensure it exists
	result, qm, err := namespaces.Info(ns.Name, nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.NotNil(t, result)
	must.Eq(t, ns.Name, result.Name)
}

func TestNamespaces_Delete(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	namespaces := c.Namespaces()

	// Create a namespace and register it
	ns := testNamespace()
	wm, err := namespaces.Register(ns, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the namespace back out again
	resp, qm, err := namespaces.List(nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 2, resp)
	must.Eq(t, ns.Name, resp[0].Name)
	must.Eq(t, "default", resp[1].Name)

	// Delete the namespace
	wm, err = namespaces.Delete(ns.Name, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the namespaces back out again
	resp, qm, err = namespaces.List(nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 1, resp)
	must.Eq(t, "default", resp[0].Name)
}

func TestNamespaces_List(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	namespaces := c.Namespaces()

	// Create two namespaces and register them
	ns1 := testNamespace()
	ns2 := testNamespace()
	ns1.Name = "fooaaa"
	ns2.Name = "foobbb"
	wm, err := namespaces.Register(ns1, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	wm, err = namespaces.Register(ns2, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the namespaces
	resp, qm, err := namespaces.List(nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 3, resp)

	// Query the namespaces using a prefix
	resp, qm, err = namespaces.PrefixList("foo", nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 2, resp)

	// Query the namespaces using a prefix
	resp, qm, err = namespaces.PrefixList("foob", nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 1, resp)
	must.Eq(t, ns2.Name, resp[0].Name)
}
