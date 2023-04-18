package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestSearch_PrefixSearch(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	job := testJob()
	_, _, err := c.Jobs().Register(job, nil)
	must.NoError(t, err)

	id := *job.ID
	prefix := id[:len(id)-2]
	resp, qm, err := c.Search().PrefixSearch(prefix, contexts.Jobs, nil)
	must.NoError(t, err)
	must.NotNil(t, qm)
	must.NotNil(t, resp)

	jobMatches := resp.Matches[contexts.Jobs]
	must.Len(t, 1, jobMatches)
	must.Eq(t, id, jobMatches[0])
}

func TestSearch_FuzzySearch(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	job := testJob()
	_, _, err := c.Jobs().Register(job, nil)
	must.NoError(t, err)

	resp, qm, err := c.Search().FuzzySearch("bin", contexts.All, nil)
	must.NoError(t, err)
	must.NotNil(t, qm)
	must.NotNil(t, resp)

	commandMatches := resp.Matches[contexts.Commands]
	must.Len(t, 1, commandMatches)
	must.Eq(t, "/bin/sleep", commandMatches[0].ID)
	must.Eq(t, []string{"default", *job.ID, "group1", "task1"}, commandMatches[0].Scope)
}
