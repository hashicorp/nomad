// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestJobEndpoint_Statuses(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, cb, func(s *TestAgent) {
		apiPath := "/v1/jobs/statuses"

		parent := mock.MinJob()
		parent.ID = "parent"
		child := mock.MinJob()
		child.ID = "parent/child"
		child.ParentID = "parent"
		otherNS := mock.MinJob()
		otherNS.ID = "otherNS"
		otherNS.Namespace = "other"

		// lil helpers
		registerJob := func(t *testing.T, job *structs.Job) {
			must.NoError(t, s.Agent.RPC("Job.Register",
				&structs.JobRegisterRequest{
					Job: job,
					WriteRequest: structs.WriteRequest{
						Region:    "global",
						Namespace: job.Namespace,
					},
				}, &structs.JobRegisterResponse{}),
			)
		}
		createNamespace := func(t *testing.T, ns string) {
			must.NoError(t, s.Agent.RPC("Namespace.UpsertNamespaces",
				&structs.NamespaceUpsertRequest{
					Namespaces: []*structs.Namespace{{
						Name: ns,
					}},
					WriteRequest: structs.WriteRequest{Region: "global"},
				}, &structs.GenericResponse{}))
		}
		buildRequest := func(t *testing.T, method, url, body string) *http.Request {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			t.Cleanup(cancel)
			var reqBody io.Reader = http.NoBody
			if body != "" {
				reqBody = bytes.NewReader([]byte(body))
			}
			req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
			must.NoError(t, err)
			return req
		}

		// note: this api will return jobs ordered by ModifyIndex,
		// so in reverse order of their creation here.
		registerJob(t, parent)
		registerJob(t, child)
		createNamespace(t, otherNS.Namespace)
		registerJob(t, otherNS)

		testCases := []struct {
			name string

			// request
			method, params, body string

			// response
			expectCode    int
			expectErr     string
			expectIDs     []string
			expectHeaders []string
		}{
			{
				name: "bad method", method: "LOL",
				expectCode: 405, expectErr: ErrInvalidMethod,
			},
			{
				name:       "bad request param",
				params:     "?include_children=not-a-bool",
				expectCode: 400, expectErr: `Failed to parse value of "include_children"`,
			},

			{
				name:      "get ok",
				expectIDs: []string{"parent"},
			},
			{
				name:      "get all namespaces",
				params:    "?namespace=*",
				expectIDs: []string{"otherNS", "parent"},
			},
			{
				name:      "get all reverse",
				params:    "?namespace=*&reverse=true",
				expectIDs: []string{"parent", "otherNS"},
			},
			{
				name:          "get one page",
				params:        "?namespace=*&per_page=1",
				expectIDs:     []string{"otherNS"},
				expectHeaders: []string{"X-Nomad-NextToken"},
			},
			{
				name:      "get children",
				params:    "?include_children=true",
				expectIDs: []string{"parent/child", "parent"},
			},
			{
				name: "get children filter",
				// this is how the UI does parent job pages
				params:    "?include_children=true&filter=ParentID == parent",
				expectIDs: []string{"parent/child"},
			},

			// POST and GET are interchangeable, but by convention, the UI will
			// POST when sending a request body, so here we test like that too.
			{
				name:       "post no jobs",
				method:     "POST",
				body:       `{"jobs": []}`,
				expectCode: 400, expectErr: "no jobs in request",
			},
			{
				name:   "post bad body",
				method: "POST", body: "{malformed",
				expectCode: 400, expectErr: "error decoding request: invalid character 'm'",
			},
			{
				name:      "post nonexistent job",
				method:    "POST",
				body:      `{"jobs": [{"id": "whatever", "namespace": "nope"}]}`,
				expectIDs: []string{},
			},
			{
				name:      "post single job",
				method:    "POST",
				body:      `{"jobs": [{"id": "parent"}]}`,
				expectIDs: []string{"parent"},
			},
			{
				name:   "post all namespaces",
				method: "POST",
				// no ?namespace param required, because we default to "*"
				// if there is a request body (and ns query is "default")
				body:      `{"jobs": [{"id": "parent"}, {"id": "otherNS", "namespace": "other"}]}`,
				expectIDs: []string{"otherNS", "parent"},
			},
			{
				name:   "post auto namespace",
				method: "POST",
				// namespace gets overridden by the RPC endpoint,
				// because jobs in the request body are all one namespace.
				params:    "?namespace=nope",
				body:      `{"jobs": [{"id": "parent", "namespace": "default"}]}`,
				expectIDs: []string{"parent"},
			},
			{
				name:   "post auto namespaces other",
				method: "POST",
				// "other" namespace should be auto-detected, as it's the only one
				body:      `{"jobs": [{"id": "otherNS", "namespace": "other"}]}`,
				expectIDs: []string{"otherNS"},
			},
			{
				name:   "post wrong namespace param",
				method: "POST",
				params: "?namespace=nope",
				// namespace can not be auto-detected, since there are two here,
				// so it uses the provided param
				body:      `{"jobs": [{"id": "parent"}, {"id": "otherNS", "namespace": "other"}]}`,
				expectIDs: []string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// default happy path values
				if tc.method == "" {
					tc.method = "GET"
				}
				if tc.expectCode == 0 {
					tc.expectCode = 200
				}

				req := buildRequest(t, tc.method, apiPath+tc.params, tc.body)
				recorder := httptest.NewRecorder()

				// method under test!
				raw, err := s.Server.JobStatusesRequest(recorder, req)

				// sad path
				if tc.expectErr != "" {
					must.ErrorContains(t, err, tc.expectErr)
					var coded *codedError
					must.True(t, errors.As(err, &coded))
					must.Eq(t, tc.expectCode, coded.code)

					must.Nil(t, raw)
					return
				}

				// happy path
				must.NoError(t, err)
				result := recorder.Result()
				must.Eq(t, tc.expectCode, result.StatusCode)

				// check response body
				jobs := raw.([]structs.JobStatusesJob)
				gotIDs := make([]string, len(jobs))
				for i, j := range jobs {
					gotIDs[i] = j.ID
				}
				must.Eq(t, tc.expectIDs, gotIDs)

				// check headers
				expectHeaders := append(
					[]string{
						"X-Nomad-Index",
						"X-Nomad-Lastcontact",
						"X-Nomad-Knownleader",
					},
					tc.expectHeaders...,
				)
				for _, h := range expectHeaders {
					test.NotEq(t, "", result.Header.Get(h),
						test.Sprintf("expect '%s' header", h))
				}
			})
		}
	})
}
