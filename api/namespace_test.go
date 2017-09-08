// +build pro ent

package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNamespaces_Register(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	namespaces := c.Namespaces()

	// Create a namespace and register it
	ns := testNamespace()
	wm, err := namespaces.Register(ns, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	// Query the jobs back out again
	resp, qm, err := namespaces.List(nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 2)
	assert.Equal(ns.Name, resp[0].Name)
	assert.Equal("default", resp[1].Name)
}

func TestNamespaces_Register_Invalid(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	namespaces := c.Namespaces()

	// Create an invalid namespace and register it
	ns := testNamespace()
	ns.Name = "*"
	_, err := namespaces.Register(ns, nil)
	assert.NotNil(err)
}

func TestNamespace_Info(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	namespaces := c.Namespaces()

	// Trying to retrieve a namespace before it exists returns an error
	_, _, err := namespaces.Info("foo", nil)
	assert.Nil(err)
	assert.Contains("not found", err.Error())

	// Register the namespace
	ns := testNamespace()
	wm, err := namespaces.Register(ns, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	// Query the namespace again and ensure it exists
	result, qm, err := namespaces.Info(ns.Name, nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.NotNil(result)
	assert.Equal(ns.Name, result.Name)
}

func TestNamespaces_Delete(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	namespaces := c.Namespaces()

	// Create a namespace and register it
	ns := testNamespace()
	wm, err := namespaces.Register(ns, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	// Query the namespace back out again
	resp, qm, err := namespaces.List(nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 2)
	assert.Equal(ns.Name, resp[0].Name)
	assert.Equal("default", resp[1].Name)

	// Delete the namespace
	wm, err = namespaces.Delete(ns.Name, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	// Query the namespaces back out again
	resp, qm, err = namespaces.List(nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 1)
	assert.Equal("default", resp[0].Name)
}

func TestNamespaces_List(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	namespaces := c.Namespaces()

	// Create two namespaces and register them
	ns1 := testNamespace()
	ns2 := testNamespace()
	ns1.Name = "fooaaa"
	ns2.Name = "foobbb"
	wm, err := namespaces.Register(ns1, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	wm, err = namespaces.Register(ns2, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	// Query the namespaces
	resp, qm, err := namespaces.List(nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 3)

	// Query the namespaces using a prefix
	resp, qm, err = namespaces.PrefixList("foo", nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 2)

	// Query the namespaces using a prefix
	resp, qm, err = namespaces.PrefixList("foob", nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 1)
	assert.Equal(ns2.Name, resp[0].Name)
}
