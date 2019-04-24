package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	// NodeDownErr marks an operation as not able to complete since the node is
	// down.
	NodeDownErr = fmt.Errorf("node down")
)

const (
	AllocDesiredStatusRun   = "run"   // Allocation should run
	AllocDesiredStatusStop  = "stop"  // Allocation should stop
	AllocDesiredStatusEvict = "evict" // Allocation should stop, and was evicted
)

const (
	AllocClientStatusPending  = "pending"
	AllocClientStatusRunning  = "running"
	AllocClientStatusComplete = "complete"
	AllocClientStatusFailed   = "failed"
	AllocClientStatusLost     = "lost"
)

// Allocations is used to query the alloc-related endpoints.
type Allocations struct {
	client *Client
}

// Allocations returns a handle on the allocs endpoints.
func (c *Client) Allocations() *Allocations {
	return &Allocations{client: c}
}

// List returns a list of all of the allocations.
func (a *Allocations) List(q *QueryOptions) ([]*AllocationListStub, *QueryMeta, error) {
	var resp []*AllocationListStub
	qm, err := a.client.query("/v1/allocations", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	sort.Sort(AllocIndexSort(resp))
	return resp, qm, nil
}

func (a *Allocations) PrefixList(prefix string) ([]*AllocationListStub, *QueryMeta, error) {
	return a.List(&QueryOptions{Prefix: prefix})
}

// Info is used to retrieve a single allocation.
func (a *Allocations) Info(allocID string, q *QueryOptions) (*Allocation, *QueryMeta, error) {
	var resp Allocation
	qm, err := a.client.query("/v1/allocation/"+allocID, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

func (a *Allocations) Exec(ctx context.Context,
	alloc *Allocation, task string, tty bool, command []string,
	stdin io.Reader, stdout, stderr io.Writer,
	terminalSizeCh <-chan TerminalSize, q *QueryOptions) (exitCode int, err error) {

	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	errCh := make(chan error, 1)

	sender, output := a.execFrames(ctx, alloc, task, tty, command, errCh, q)

	select {
	case err := <-errCh:
		return -1, err
	default:
	}

	// forwarding stdin
	go func() {

		bytes := make([]byte, 2048)
		for {
			if ctx.Err() != nil {
				return
			}

			input := ExecStreamingInput{Stdin: &FileOperation{}}

			n, err := stdin.Read(bytes)
			if err == io.EOF {
				input.Stdin.Close = true
				sender(&input)
				return
			} else if err != nil {
				errCh <- err
				return
			}

			input.Stdin.Data = bytes[:n]
			sender(&input)
		}
	}()

	// forwarding terminal size and heartbeats
	go func() {
		for {
			resizeInput := ExecStreamingInput{}

			select {
			case <-ctx.Done():
				return
			case size, ok := <-terminalSizeCh:
				if !ok {
					continue
				}
				resizeInput.TTYSize = &size
				sender(&resizeInput)

			// heartbeat message
			case <-time.After(10 * time.Second):
				sender(&ExecStreamingInputHeartbeat)
			}

		}
	}()

	for {
		select {
		case err := <-errCh:
			// drop websocket code, not relevant to user
			if wsErr, ok := err.(*websocket.CloseError); ok && wsErr.Text != "" {
				return -1, errors.New(wsErr.Text)
			}
			return -1, err
		case <-ctx.Done():
			return -1, ctx.Err()
		case frame, ok := <-output:
			if !ok {
				return -1, nil
			}

			switch {
			case frame.Stdout != nil && len(frame.Stdout.Data) != 0:
				stdout.Write(frame.Stdout.Data)
			case frame.Stderr != nil && len(frame.Stderr.Data) != 0:
				stderr.Write(frame.Stderr.Data)
			case frame.Stdout != nil && frame.Stdout.Close:
				// don't really do anything
			case frame.Stderr != nil && frame.Stderr.Close:
				// don't really do anything
			case frame.Exited && frame.Result != nil:
				return frame.Result.ExitCode, nil
			default:
				// unexpected event, TODO: log it?!
			}
		}
	}
}

func (a *Allocations) execFrames(ctx context.Context, alloc *Allocation, task string, tty bool, command []string,
	errCh chan<- error, q *QueryOptions) (sendFn func(interface{}) error, output <-chan *ExecStreamingOutput) {

	nodeClient, err := a.client.GetNodeClientWithTimeout(alloc.NodeID, ClientConnTimeout, q)
	if err != nil {
		errCh <- err
		return nil, nil
	}

	if q == nil {
		q = &QueryOptions{}
	}
	if q.Params == nil {
		q.Params = make(map[string]string)
	}

	commandBytes, err := json.Marshal(command)
	if err != nil {
		errCh <- fmt.Errorf("failed to marshal command: %s", err)
		return nil, nil
	}

	q.Params["tty"] = strconv.FormatBool(tty)
	q.Params["task"] = task
	q.Params["command"] = string(commandBytes)

	reqPath := fmt.Sprintf("/v1/client/allocation/%s/exec", alloc.ID)

	conn, _, err := nodeClient.websocket(reqPath, q)
	if err != nil {
		// There was a networking error when talking directly to the client.
		if _, ok := err.(net.Error); !ok {
			errCh <- err
			return nil, nil
		}

		conn, _, err = a.client.websocket(reqPath, q)
		if err != nil {
			errCh <- err
			return nil, nil
		}
	}

	// Create the output channel
	frames := make(chan *ExecStreamingOutput, 10)

	go func() {
		defer conn.Close()
		for {
			// Check if we have been cancelled
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Decode the next frame
			var frame ExecStreamingOutput
			err := conn.ReadJSON(&frame)
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				close(frames)
				return
			} else if err != nil {
				errCh <- err
				return
			}

			frames <- &frame
		}
	}()

	var sendLock sync.Mutex
	send := func(v interface{}) error {
		sendLock.Lock()
		defer sendLock.Unlock()

		return conn.WriteJSON(v)
	}

	return send, frames

}

func (a *Allocations) Stats(alloc *Allocation, q *QueryOptions) (*AllocResourceUsage, error) {
	var resp AllocResourceUsage
	path := fmt.Sprintf("/v1/client/allocation/%s/stats", alloc.ID)
	_, err := a.client.query(path, &resp, q)
	return &resp, err
}

func (a *Allocations) GC(alloc *Allocation, q *QueryOptions) error {
	nodeClient, err := a.client.GetNodeClient(alloc.NodeID, q)
	if err != nil {
		return err
	}

	var resp struct{}
	_, err = nodeClient.query("/v1/client/allocation/"+alloc.ID+"/gc", &resp, nil)
	return err
}

func (a *Allocations) Restart(alloc *Allocation, taskName string, q *QueryOptions) error {
	req := AllocationRestartRequest{
		TaskName: taskName,
	}

	var resp struct{}
	_, err := a.client.putQuery("/v1/client/allocation/"+alloc.ID+"/restart", &req, &resp, q)
	return err
}

func (a *Allocations) Stop(alloc *Allocation, q *QueryOptions) (*AllocStopResponse, error) {
	var resp AllocStopResponse
	_, err := a.client.putQuery("/v1/allocation/"+alloc.ID+"/stop", nil, &resp, q)
	return &resp, err
}

// AllocStopResponse is the response to an `AllocStopRequest`
type AllocStopResponse struct {
	// EvalID is the id of the follow up evalution for the rescheduled alloc.
	EvalID string

	WriteMeta
}

func (a *Allocations) Signal(alloc *Allocation, q *QueryOptions, task, signal string) error {
	nodeClient, err := a.client.GetNodeClient(alloc.NodeID, q)
	if err != nil {
		return err
	}

	req := AllocSignalRequest{
		AllocID: alloc.ID,
		Signal:  signal,
		Task:    task,
	}

	var resp GenericResponse
	_, err = nodeClient.putQuery("/v1/client/allocation/"+alloc.ID+"/signal", &req, &resp, q)
	return err
}

type FileOperation struct {
	Data  []byte `json:"data,omitempty"`
	Close bool   `json:"close,omitempty"`
}

type TerminalSize struct {
	Height int32 `json:"height,omitempty"`
	Width  int32 `json:"width,omitempty"`
}

var ExecStreamingInputHeartbeat = ExecStreamingInput{}

type ExecStreamingInput struct {
	Stdin   *FileOperation `json:"stdin,omitempty"`
	TTYSize *TerminalSize  `json:"tty_size,omitempty"`
}

type ExecStreamingExitResult struct {
	ExitCode int `json:"exit_code"`
}
type ExecStreamingOutput struct {
	Stdout *FileOperation `json:"stdout,omitempty"`
	Stderr *FileOperation `json:"stderr,omitempty"`

	Exited bool                     `json:"exited,omitempty"`
	Result *ExecStreamingExitResult `json:"result,omitempty"`
}

// Allocation is used for serialization of allocations.
type Allocation struct {
	ID                    string
	Namespace             string
	EvalID                string
	Name                  string
	NodeID                string
	NodeName              string
	JobID                 string
	Job                   *Job
	TaskGroup             string
	Resources             *Resources
	TaskResources         map[string]*Resources
	AllocatedResources    *AllocatedResources
	Services              map[string]string
	Metrics               *AllocationMetric
	DesiredStatus         string
	DesiredDescription    string
	DesiredTransition     DesiredTransition
	ClientStatus          string
	ClientDescription     string
	TaskStates            map[string]*TaskState
	DeploymentID          string
	DeploymentStatus      *AllocDeploymentStatus
	FollowupEvalID        string
	PreviousAllocation    string
	NextAllocation        string
	RescheduleTracker     *RescheduleTracker
	PreemptedAllocations  []string
	PreemptedByAllocation string
	CreateIndex           uint64
	ModifyIndex           uint64
	AllocModifyIndex      uint64
	CreateTime            int64
	ModifyTime            int64
}

// AllocationMetric is used to deserialize allocation metrics.
type AllocationMetric struct {
	NodesEvaluated     int
	NodesFiltered      int
	NodesAvailable     map[string]int
	ClassFiltered      map[string]int
	ConstraintFiltered map[string]int
	NodesExhausted     int
	ClassExhausted     map[string]int
	DimensionExhausted map[string]int
	QuotaExhausted     []string
	// Deprecated, replaced with ScoreMetaData
	Scores            map[string]float64
	AllocationTime    time.Duration
	CoalescedFailures int
	ScoreMetaData     []*NodeScoreMeta
}

// NodeScoreMeta is used to serialize node scoring metadata
// displayed in the CLI during verbose mode
type NodeScoreMeta struct {
	NodeID    string
	Scores    map[string]float64
	NormScore float64
}

// AllocationListStub is used to return a subset of an allocation
// during list operations.
type AllocationListStub struct {
	ID                    string
	EvalID                string
	Name                  string
	Namespace             string
	NodeID                string
	NodeName              string
	JobID                 string
	JobType               string
	JobVersion            uint64
	TaskGroup             string
	DesiredStatus         string
	DesiredDescription    string
	ClientStatus          string
	ClientDescription     string
	TaskStates            map[string]*TaskState
	DeploymentStatus      *AllocDeploymentStatus
	FollowupEvalID        string
	RescheduleTracker     *RescheduleTracker
	PreemptedAllocations  []string
	PreemptedByAllocation string
	CreateIndex           uint64
	ModifyIndex           uint64
	CreateTime            int64
	ModifyTime            int64
}

// AllocDeploymentStatus captures the status of the allocation as part of the
// deployment. This can include things like if the allocation has been marked as
// healthy.
type AllocDeploymentStatus struct {
	Healthy     *bool
	Timestamp   time.Time
	Canary      bool
	ModifyIndex uint64
}

type AllocatedResources struct {
	Tasks  map[string]*AllocatedTaskResources
	Shared AllocatedSharedResources
}

type AllocatedTaskResources struct {
	Cpu      AllocatedCpuResources
	Memory   AllocatedMemoryResources
	Networks []*NetworkResource
}

type AllocatedSharedResources struct {
	DiskMB int64
}

type AllocatedCpuResources struct {
	CpuShares int64
}

type AllocatedMemoryResources struct {
	MemoryMB int64
}

// AllocIndexSort reverse sorts allocs by CreateIndex.
type AllocIndexSort []*AllocationListStub

func (a AllocIndexSort) Len() int {
	return len(a)
}

func (a AllocIndexSort) Less(i, j int) bool {
	return a[i].CreateIndex > a[j].CreateIndex
}

func (a AllocIndexSort) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// RescheduleInfo is used to calculate remaining reschedule attempts
// according to the given time and the task groups reschedule policy
func (a Allocation) RescheduleInfo(t time.Time) (int, int) {
	var reschedulePolicy *ReschedulePolicy
	for _, tg := range a.Job.TaskGroups {
		if *tg.Name == a.TaskGroup {
			reschedulePolicy = tg.ReschedulePolicy
		}
	}
	if reschedulePolicy == nil {
		return 0, 0
	}
	availableAttempts := *reschedulePolicy.Attempts
	interval := *reschedulePolicy.Interval
	attempted := 0

	// Loop over reschedule tracker to find attempts within the restart policy's interval
	if a.RescheduleTracker != nil && availableAttempts > 0 && interval > 0 {
		for j := len(a.RescheduleTracker.Events) - 1; j >= 0; j-- {
			lastAttempt := a.RescheduleTracker.Events[j].RescheduleTime
			timeDiff := t.UTC().UnixNano() - lastAttempt
			if timeDiff < interval.Nanoseconds() {
				attempted += 1
			}
		}
	}
	return attempted, availableAttempts
}

type AllocationRestartRequest struct {
	TaskName string
}

type AllocSignalRequest struct {
	AllocID string
	Task    string
	Signal  string
}

// GenericResponse is used to respond to a request where no
// specific response information is needed.
type GenericResponse struct {
	WriteMeta
}

// RescheduleTracker encapsulates previous reschedule events
type RescheduleTracker struct {
	Events []*RescheduleEvent
}

// RescheduleEvent is used to keep track of previous attempts at rescheduling an allocation
type RescheduleEvent struct {
	// RescheduleTime is the timestamp of a reschedule attempt
	RescheduleTime int64

	// PrevAllocID is the ID of the previous allocation being restarted
	PrevAllocID string

	// PrevNodeID is the node ID of the previous allocation
	PrevNodeID string
}

// DesiredTransition is used to mark an allocation as having a desired state
// transition. This information can be used by the scheduler to make the
// correct decision.
type DesiredTransition struct {
	// Migrate is used to indicate that this allocation should be stopped and
	// migrated to another node.
	Migrate *bool

	// Reschedule is used to indicate that this allocation is eligible to be
	// rescheduled.
	Reschedule *bool
}

// ShouldMigrate returns whether the transition object dictates a migration.
func (d DesiredTransition) ShouldMigrate() bool {
	return d.Migrate != nil && *d.Migrate
}
