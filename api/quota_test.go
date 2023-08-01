//go:build ent

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestQuotas_Register(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Create a quota spec and register it
	qs := testQuotaSpec()
	wm, err := quotas.Register(qs, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the specs back out again
	resp, qm, err := quotas.List(nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 1, resp)
	must.Eq(t, qs.Name, resp[0].Name)
}

func TestQuotas_Register_Invalid(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Create an invalid namespace and register it
	qs := testQuotaSpec()
	qs.Name = "*"
	_, err := quotas.Register(qs, nil)
	must.Error(t, err)
}

func TestQuotas_Info(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Trying to retrieve a quota spec before it exists returns an error
	_, _, err := quotas.Info("foo", nil)
	must.ErrorContains(t, err, "not found")

	// Register the quota
	qs := testQuotaSpec()
	wm, err := quotas.Register(qs, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the quota again and ensure it exists
	result, qm, err := quotas.Info(qs.Name, nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.NotNil(t, result)
	must.Eq(t, qs.Name, result.Name)
}

func TestQuotas_Usage(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Trying to retrieve a quota spec before it exists returns an error
	_, _, err := quotas.Usage("foo", nil)
	must.ErrorContains(t, err, "not found")

	// Register the quota
	qs := testQuotaSpec()
	wm, err := quotas.Register(qs, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the quota usage and ensure it exists
	result, qm, err := quotas.Usage(qs.Name, nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.NotNil(t, result)
	must.Eq(t, qs.Name, result.Name)
}

func TestQuotas_Delete(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Create a quota and register it
	qs := testQuotaSpec()
	wm, err := quotas.Register(qs, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the quota back out again
	resp, qm, err := quotas.List(nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 1, resp)
	must.Eq(t, qs.Name, resp[0].Name)

	// Delete the quota
	wm, err = quotas.Delete(qs.Name, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the quotas back out again
	resp, qm, err = quotas.List(nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.SliceEmpty(t, resp)
}

func TestQuotas_List(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Create two quotas and register them
	qs1 := testQuotaSpec()
	qs2 := testQuotaSpec()
	qs1.Name = "fooaaa"
	qs2.Name = "foobbb"
	wm, err := quotas.Register(qs1, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	wm, err = quotas.Register(qs2, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the quotas
	resp, qm, err := quotas.List(nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 2, resp)

	// Query the quotas using a prefix
	resp, qm, err = quotas.PrefixList("foo", nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 2, resp)

	// Query the quotas using a prefix
	resp, qm, err = quotas.PrefixList("foob", nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 1, resp)
	must.Eq(t, qs2.Name, resp[0].Name)
}

func TestQuotas_ListUsages(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	quotas := c.Quotas()

	// Create two quotas and register them
	qs1 := testQuotaSpec()
	qs2 := testQuotaSpec()
	qs1.Name = "fooaaa"
	qs2.Name = "foobbb"
	wm, err := quotas.Register(qs1, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	wm, err = quotas.Register(qs2, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the quotas
	resp, qm, err := quotas.ListUsage(nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 2, resp)

	// Query the quotas using a prefix
	resp, qm, err = quotas.PrefixListUsage("foo", nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 2, resp)

	// Query the quotas using a prefix
	resp, qm, err = quotas.PrefixListUsage("foob", nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 1, resp)
	must.Eq(t, qs2.Name, resp[0].Name)
}
