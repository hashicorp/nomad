package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/stretchr/testify/require"
)

func TestSearch_PrefixSearch(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	job := testJob()
	_, _, err := c.Jobs().Register(job, nil)
	require.NoError(t, err)

	id := *job.ID
	prefix := id[:len(id)-2]
	resp, qm, err := c.Search().PrefixSearch(prefix, contexts.Jobs, nil)
	require.NoError(t, err)
	require.NotNil(t, qm)
	require.NotNil(t, resp)

	jobMatches := resp.Matches[contexts.Jobs]
	require.Len(t, jobMatches, 1)
	require.Equal(t, id, jobMatches[0])
}

func TestSearch_FuzzySearch(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	job := testJob()
	_, _, err := c.Jobs().Register(job, nil)
	require.NoError(t, err)

	resp, qm, err := c.Search().FuzzySearch("bin", contexts.All, nil)
	require.NoError(t, err)
	require.NotNil(t, qm)
	require.NotNil(t, resp)

	commandMatches := resp.Matches[contexts.Commands]
	require.Len(t, commandMatches, 1)
	require.Equal(t, "/bin/sleep", commandMatches[0].ID)
	require.Equal(t, []string{
		"default", *job.ID, "group1", "task1",
	}, commandMatches[0].Scope)
}
