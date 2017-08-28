package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/stretchr/testify/assert"
)

func TestSearch_List(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	job := testJob()
	_, _, err := c.Jobs().Register(job, nil)
	assert.Nil(err)

	id := *job.ID
	prefix := id[:len(id)-2]
	resp, qm, err := c.Search().PrefixSearch(prefix, contexts.Jobs, nil)

	assert.Nil(err)
	assert.NotNil(qm)

	jobMatches := resp.Matches[contexts.Jobs]
	assert.Equal(1, len(jobMatches))
	assert.Equal(id, jobMatches[0])
}
