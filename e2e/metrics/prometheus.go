// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/prometheus/common/model"
)

func (tc *MetricsTest) setUpPrometheus(f *framework.F) error {
	uuid := uuid.Generate()
	fabioID := "fabio" + uuid[0:8]
	fabioAllocs := e2eutil.RegisterAndWaitForAllocs(f.T(), tc.Nomad(),
		"metrics/input/fabio.nomad", fabioID, "")
	if len(fabioAllocs) < 1 {
		return fmt.Errorf("fabio failed to start")
	}
	tc.fabioID = fabioID

	// get a fabio IP address so we can query it later
	nodeDetails, _, err := tc.Nomad().Nodes().Info(fabioAllocs[0].NodeID, nil)
	if err != nil {
		return err
	}

	// TODO(tgross): currently this forces us to run the target on AWS rather
	// than any other environment. There's a Provider environment in the E2E
	// framework we're not currently using; we should revisit that.
	publicIP := nodeDetails.Attributes["unique.platform.aws.public-ipv4"]
	tc.fabioAddress = fmt.Sprintf("http://%s:9999", publicIP)
	prometheusID := "prometheus" + uuid[0:8]
	prometheusAllocs := e2eutil.RegisterAndWaitForAllocs(f.T(), tc.Nomad(),
		"metrics/input/prometheus.nomad", prometheusID, "")
	if len(prometheusAllocs) < 1 {
		return fmt.Errorf("prometheus failed to start")
	}
	tc.prometheusID = prometheusID
	return nil
}

func (tc *MetricsTest) tearDownPrometheus(f *framework.F) {
	tc.Nomad().Jobs().Deregister(tc.prometheusID, true, nil)
	tc.Nomad().Jobs().Deregister(tc.fabioID, true, nil)
	tc.Nomad().System().GarbageCollect()
}

// "Wait, why aren't we just using the prometheus golang client?", you ask?
// Nomad has vendored an older version of the prometheus exporter library
// their HTTP client which only works with a newer version is also is marked
// "alpha", and there's API v2 work currently ongoing. Rather than waiting
// till 0.11 to ship this test, this just handles the query API and can be
// swapped out later.
//
// TODO(tgross) / COMPAT(0.11): update our prometheus libraries
func (tc *MetricsTest) promQuery(query string) (model.Vector, error) {
	var err error
	promUrl := tc.fabioAddress + "/api/v1/query"
	formValues := url.Values{"query": {query}}
	resp, err := http.PostForm(promUrl, formValues)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status: %v", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	apiResp := &apiResponse{}
	err = json.Unmarshal(body, apiResp)
	if err != nil {
		return nil, err
	}
	if apiResp.Status == "error" {
		return nil, fmt.Errorf("API error: %v: %v", apiResp.ErrorType, apiResp.Error)
	}

	// unpack query
	var qs queryResult
	err = json.Unmarshal(apiResp.Data, &qs)
	if err != nil {
		return nil, err
	}
	val, ok := qs.v.(model.Vector)
	if !ok || len(val) == 0 {
		return nil, fmt.Errorf("no metrics data available")
	}
	return val, nil
}

type apiResponse struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	ErrorType string          `json:"errorType"`
	Error     string          `json:"error"`
	Warnings  []string        `json:"warnings,omitempty"`
}

// queryResult contains result data for a query.
type queryResult struct {
	Type   model.ValueType `json:"resultType"`
	Result interface{}     `json:"result"`

	// The decoded value.
	v model.Value
}

func (qr *queryResult) UnmarshalJSON(b []byte) error {
	v := struct {
		Type   model.ValueType `json:"resultType"`
		Result json.RawMessage `json:"result"`
	}{}

	err := json.Unmarshal(b, &v)
	if err != nil {
		return err
	}

	switch v.Type {
	case model.ValScalar:
		var sv model.Scalar
		err = json.Unmarshal(v.Result, &sv)
		qr.v = &sv

	case model.ValVector:
		var vv model.Vector
		err = json.Unmarshal(v.Result, &vv)
		qr.v = vv

	case model.ValMatrix:
		var mv model.Matrix
		err = json.Unmarshal(v.Result, &mv)
		qr.v = mv

	default:
		err = fmt.Errorf("no metrics data available")
	}
	return err
}
