//go:build ent
// +build ent

package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuotas_Register(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Create a quota spec and register it
	qs := testQuotaSpec()
	wm, err := quotas.Register(qs, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	// Query the specs back out again
	resp, qm, err := quotas.List(nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 1)
	assert.Equal(qs.Name, resp[0].Name)
}

func TestQuotas_Register_Invalid(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Create an invalid namespace and register it
	qs := testQuotaSpec()
	qs.Name = "*"
	_, err := quotas.Register(qs, nil)
	assert.NotNil(err)
}

func TestQuotas_Info(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Trying to retrieve a quota spec before it exists returns an error
	_, _, err := quotas.Info("foo", nil)
	assert.NotNil(err)
	assert.Contains(err.Error(), "not found")

	// Register the quota
	qs := testQuotaSpec()
	wm, err := quotas.Register(qs, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	// Query the quota again and ensure it exists
	result, qm, err := quotas.Info(qs.Name, nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.NotNil(result)
	assert.Equal(qs.Name, result.Name)
}

func TestQuotas_Usage(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Trying to retrieve a quota spec before it exists returns an error
	_, _, err := quotas.Usage("foo", nil)
	assert.NotNil(err)
	assert.Contains(err.Error(), "not found")

	// Register the quota
	qs := testQuotaSpec()
	wm, err := quotas.Register(qs, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	// Query the quota usage and ensure it exists
	result, qm, err := quotas.Usage(qs.Name, nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.NotNil(result)
	assert.Equal(qs.Name, result.Name)
}

func TestQuotas_Delete(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Create a quota and register it
	qs := testQuotaSpec()
	wm, err := quotas.Register(qs, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	// Query the quota back out again
	resp, qm, err := quotas.List(nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 1)
	assert.Equal(qs.Name, resp[0].Name)

	// Delete the quota
	wm, err = quotas.Delete(qs.Name, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	// Query the quotas back out again
	resp, qm, err = quotas.List(nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 0)
}

func TestQuotas_List(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Create two quotas and register them
	qs1 := testQuotaSpec()
	qs2 := testQuotaSpec()
	qs1.Name = "fooaaa"
	qs2.Name = "foobbb"
	wm, err := quotas.Register(qs1, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	wm, err = quotas.Register(qs2, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	// Query the quotas
	resp, qm, err := quotas.List(nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 2)

	// Query the quotas using a prefix
	resp, qm, err = quotas.PrefixList("foo", nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 2)

	// Query the quotas using a prefix
	resp, qm, err = quotas.PrefixList("foob", nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 1)
	assert.Equal(qs2.Name, resp[0].Name)
}

func TestQuotas_ListUsages(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Create two quotas and register them
	qs1 := testQuotaSpec()
	qs2 := testQuotaSpec()
	qs1.Name = "fooaaa"
	qs2.Name = "foobbb"
	wm, err := quotas.Register(qs1, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	wm, err = quotas.Register(qs2, nil)
	assert.Nil(err)
	assertWriteMeta(t, wm)

	// Query the quotas
	resp, qm, err := quotas.ListUsage(nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 2)

	// Query the quotas using a prefix
	resp, qm, err = quotas.PrefixListUsage("foo", nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 2)

	// Query the quotas using a prefix
	resp, qm, err = quotas.PrefixListUsage("foob", nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)
	assert.Len(resp, 1)
	assert.Equal(qs2.Name, resp[0].Name)
}
