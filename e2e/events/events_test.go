package events

import (
	"context"
	"fmt"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
	"testing"
)

// TestEventStream_Ports tests that removing the mapstructure tags from the Port
// struct results in Port values being serialized correctly to the event stream.
func TestEventStream_Ports(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 1)

	testCases := []struct {
		name     string
		networks []*structs.NetworkResource
	}{
		{
			name: "static-ports",
			networks: []*structs.NetworkResource{
				{
					ReservedPorts: []structs.Port{
						{
							Label: "http",
							Value: 9000,
						},
					},
				},
			},
		},
		{
			name: "dynamic-ports",
			networks: []*structs.NetworkResource{
				{
					DynamicPorts: []structs.Port{
						{
							Label: "http",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var jobIDs []string
			t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

			jobID := "event-stream-" + tc.name + "-" + uuid.Short()

			err := e2eutil.Register(jobID, "./input/"+tc.name+".nomad")
			require.NoError(t, err)
			jobIDs = append(jobIDs, jobID)

			err = e2eutil.WaitForAllocStatusExpected(jobID, "",
				[]string{"running"})
			require.NoError(t, err, "job should be running")

			err = e2eutil.WaitForLastDeploymentStatus(jobID, "", "successful", nil)
			require.NoError(t, err, "success", "deployment did not complete")

			topics := map[api.Topic][]string{
				api.TopicAllocation: {jobID},
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			events := nomadClient.EventStream()
			streamCh, err := events.Stream(ctx, topics, 1, nil)
			require.NoError(t, err)

			var allocEvents []api.Event
			// gather job alloc events
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case event, ok := <-streamCh:
						if !ok {
							return
						}
						if event.IsHeartbeat() {
							continue
						}
						allocEvents = append(allocEvents, event.Events...)
					}
				}
			}()

			var alloc *api.Allocation
			testutil.WaitForResult(func() (bool, error) {
				var got string
				for _, e := range allocEvents {
					if e.Type == "AllocationUpdated" {
						alloc, err = e.Allocation()
						return true, nil
					}
					got = e.Type
				}
				return false, fmt.Errorf("expected to receive allocation updated event, got: %#v", got)
			}, func(e error) {
				require.NoError(t, err)
			})

			require.NotNil(t, alloc)
			require.Len(t, alloc.Resources.Networks, 1)
			if len(tc.networks[0].ReservedPorts) == 1 {
				require.Equal(t, tc.networks[0].ReservedPorts[0].Value, alloc.Resources.Networks[0].ReservedPorts[0].Value)
			}

			if len(tc.networks[0].DynamicPorts) == 1 {
				require.NotEqual(t, 0, alloc.Resources.Networks[0].DynamicPorts[0].Value)
			}
		})
	}
}
