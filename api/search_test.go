package api

import (
	"testing"

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
	resp, err := c.Search().List(prefix, "jobs")

	assert.Nil(err)
	assert.NotEqual(0, resp.Index)

	jobMatches := resp.Matches["jobs"]
	assert.Equal(1, len(jobMatches))
	assert.Equal(id, jobMatches[0])
}
