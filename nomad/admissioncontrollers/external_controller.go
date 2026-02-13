package admission

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/nomad/nomad/structs"
)

type External struct {
	name     string
	endpoint string
	nodePool string
}

func NewExternalController(name, endpoint, nodePool string) *External {
	return &External{
		name:     name,
		endpoint: endpoint,
		nodePool: nodePool,
	}
}

// Start is a noop for external controllers
func (c *External) Start(ctx context.Context) {}

func (c *External) Name() string {
	return c.name
}

// AdmitJob is used to send the job to an external process for processing.
//
// This is a basic implementation for proof of concept.
func (c *External) AdmitJob(job *structs.Job) (warnings []error, err error) {
	if job.NodePool != c.nodePool {
		return nil, nil
	}

	data, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received bad response code, %d", res.StatusCode)
	}

	newJob := &structs.Job{}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, newJob)
	if err != nil {
		return nil, err
	}

	job = newJob

	return nil, nil
}
