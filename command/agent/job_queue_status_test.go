// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestHTTP_JobsBatchQueue(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/jobs/queue/status", nil)
		must.NoError(t, err)
		respW := httptest.NewRecorder()

		// Make the request
		_, err = s.Server.JobQueueStatus(respW, req)
		must.NoError(t, err)

		req.Method = http.MethodPost
		_, err = s.Server.JobQueueStatus(respW, req)
		must.Error(t, err)

		req.Method = http.MethodPut
		_, err = s.Server.JobQueueStatus(respW, req)
		must.Error(t, err)

		req.Method = http.MethodDelete
		_, err = s.Server.JobQueueStatus(respW, req)
		must.Error(t, err)
	})
}
