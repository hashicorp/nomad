package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/stretchr/testify/require"
)

func TestSearch_List(t *testing.T) {
	require := require.New(t)
	t.Parallel()

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	job := testJob()
	_, _, err := c.Jobs().Register(job, nil)
	require.Nil(err)

	id := *job.ID
	prefix := id[:len(id)-2]
	resp, qm, err := c.Search().PrefixSearch(prefix, contexts.Jobs, nil)

	require.Nil(err)
	require.NotNil(qm)
	require.NotNil(qm)

	jobMatches := resp.Matches[contexts.Jobs]
	require.Equal(1, len(jobMatches))
	require.Equal(id, jobMatches[0])
}
