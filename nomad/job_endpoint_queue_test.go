package nomad

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/mock"
)

type MockBatchQueue struct {
	mock.Mock
}

func (m *MockBatchQueue) Enqueue(e *structs.Evaluation) {}

func (m *MockBatchQueue) Start(context.Context) error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockBatchQueue) Status() structs.QueueStatusResponse {
	args := m.Called()
	return args.Get(0).(structs.QueueStatusResponse)
}

func (m *MockBatchQueue) SetEnabled(bool, *state.StateStore) {}

// Not much to test here at the moment
func TestJob_BatchQueue(t *testing.T) {
	s, cleanup := TestServer(t, nil)
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	mockQueue := &MockBatchQueue{}
	mockQueue.On("Status", mock.Anything).Return(structs.QueueStatusResponse{
		Type:      "some_type",
		Workloads: "some_workloads",
	})
	// swap the server's queue for a mock
	s.batchJobQueue = mockQueue

	req := structs.QueueStatusRequest{QueryOptions: structs.QueryOptions{
		Region: "global",
	}}
	reply := structs.QueueStatusResponse{}

	err := s.RPC("Job.QueueStatus", &req, &reply)
	must.NoError(t, err)
	must.Eq(t, reply.Type, "some_type")
	must.Eq(t, reply.Workloads, "some_workloads")
}
