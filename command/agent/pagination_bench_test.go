// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func BenchmarkHTTP_NodesList(b *testing.B) {
	for _, count := range []int{100, 1000, 10000} {
		paginationBenchmarkHTTPTest(b, func(s *TestAgent) {
			state := s.Agent.server.State()
			for i := range count {
				must.NoError(b, state.UpsertNode(structs.MsgTypeTestSetup, uint64(1000+i), mock.Node()))
			}

			for _, perPage := range []int{0, 10, 100} {
				b.Run(fmt.Sprintf("nodes=%d/per_page=%d", count, perPage), func(b *testing.B) {
					benchmarkHTTPPagination(b, s.Server.mux, "/v1/nodes", count, perPage)
				})
			}
		})
	}
}

func BenchmarkHTTP_AllocsList(b *testing.B) {
	for _, count := range []int{1000, 10000, 100000} {
		paginationBenchmarkHTTPTest(b, func(s *TestAgent) {
			state := s.Agent.server.State()
			template := mock.Alloc()
			must.NoError(b, state.UpsertJobSummary(998, mock.JobSummary(template.JobID)))
			must.NoError(b, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, template.Job))

			allocs := make([]*structs.Allocation, count)
			for i := range allocs {
				alloc := template.CopySkipJob()
				alloc.ID = uuid.Generate()
				alloc.Name = fmt.Sprintf("%s.%s[%d]", alloc.JobID, alloc.TaskGroup, i)
				allocs[i] = alloc
			}
			must.NoError(b, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs))

			for _, perPage := range []int{0, 100, 1000} {
				b.Run(fmt.Sprintf("allocs=%d/per_page=%d", count, perPage), func(b *testing.B) {
					benchmarkHTTPPagination(b, s.Server.mux, "/v1/allocations", count, perPage)
				})
			}
		})
	}
}

// paginationBenchmarkHTTPTest discards logs to minimize interference with
// benchmark results.
func paginationBenchmarkHTTPTest(b *testing.B, f func(*TestAgent)) {
	b.Helper()
	s := &TestAgent{
		T:          b,
		Name:       b.Name(),
		Enterprise: EnterpriseTestAgent,
		logger: hclog.NewInterceptLogger(&hclog.LoggerOptions{
			Level:  hclog.Off,
			Output: io.Discard,
		}),
		ConfigCallback: func(c *Config) {
			c.Client.Enabled = false
			c.NomadConfig.LogOutput = io.Discard
		},
	}
	s.Start()
	defer s.Shutdown()
	f(s)
}

// benchmarkHTTPPagination measures a complete listing, including HTTP handling
// and JSON encoding, but not network transport or client-side JSON decoding.
// perPage == 0 omits pagination. Dataset setup and validation are not timed.
func benchmarkHTTPPagination(b *testing.B, handler http.Handler, path string, count, perPage int) {
	b.Helper()
	pages := 1
	if perPage > 0 {
		pages = (count + perPage - 1) / perPage
	}

	list := func(validate bool) {
		query := url.Values{}
		if perPage > 0 {
			query.Set("per_page", strconv.Itoa(perPage))
		}
		for page := range pages {
			req := httptest.NewRequest(http.MethodGet, path+"?"+query.Encode(), nil)
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				b.Fatalf("listing failed: HTTP %d: %s", resp.Code, resp.Body.String())
			}

			if validate {
				var items []json.RawMessage
				must.NoError(b, json.Unmarshal(resp.Body.Bytes(), &items))
				want := count
				if perPage > 0 {
					want = min(perPage, count-page*perPage)
				}
				must.Len(b, want, items)
			}

			next := resp.Header().Get("X-Nomad-NextToken")
			if (next != "") != (page < pages-1) {
				b.Fatalf("unexpected next token %q on page %d of %d", next, page+1, pages)
			}
			query.Set("next_token", next)
		}
	}

	list(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list(false)
	}
	b.StopTimer()
	b.ReportMetric(float64(pages), "requests/op")
}
