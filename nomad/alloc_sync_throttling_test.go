package nomad

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/ryanuber/columnize"
	"golang.org/x/exp/maps"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

type throttleTestNodeHandler struct {
	// This represents the state store
	store      map[string]*structs.Allocation
	storeIndex uint64
	storeLock  sync.Mutex

	// Historical data
	updatesLastBatchSize        int
	updatesLastBatchNumClients  int
	updatesLastBatchTimeToWrite time.Duration
	history                     []*batchHistory

	// note: currently the Node.UpdateAlloc handler queues up all updates, even
	// if they're redundant for a particular Allocation
	updates        []*structs.Allocation
	updatesMap     map[string]*structs.Allocation
	updatedClients map[string]struct{}
	updateFuture   *structs.BatchFuture
	updateTimer    *time.Timer
	updatesLock    sync.Mutex // must be released in RPC handler

	serverLockOnWrite bool

	serverBatchUpdateInterval time.Duration
	serverPerWrite            time.Duration
	serverBaseWriteLatency    time.Duration
}

type batchHistory struct {
	lastBatchSize        int
	lastBatchNumClients  int
	lastBatchTimeToWrite time.Duration
}

type throttleTestConfig struct {
	name            string
	numClients      int
	allocsPerClient int

	clientDynamicBackoffLast       bool
	clientBackoffThresholdLast     int
	clientDynamicBackoffCurrent    bool
	clientBackoffThresholdCurrent  int
	clientDynamicBackoffMultiplier float64
	clientNoBackoff                bool
	alwaysBackoff                  bool

	serverLockOnWrite bool

	serverBatchUpdateInterval     time.Duration
	serverPerWrite                time.Duration
	serverBaseWriteLatency        time.Duration
	clientBatchUpdateInterval     time.Duration
	clientAllocEventsBaseInterval time.Duration
}

func newThrottleTestNodeHandler(cfg *throttleTestConfig) *throttleTestNodeHandler {
	return &throttleTestNodeHandler{
		store:                     map[string]*structs.Allocation{},
		updates:                   []*structs.Allocation{},
		updatesMap:                map[string]*structs.Allocation{},
		updatedClients:            map[string]struct{}{},
		serverLockOnWrite:         cfg.serverLockOnWrite,
		serverBatchUpdateInterval: cfg.serverBatchUpdateInterval,
		serverPerWrite:            cfg.serverPerWrite,
		serverBaseWriteLatency:    cfg.serverBaseWriteLatency,
		history:                   []*batchHistory{},
	}
}

func (n *throttleTestNodeHandler) NodeUpdateAlloc(ctx context.Context, req *structs.AllocUpdateRequest, reply *throttleTestResponse) error {
	if len(req.Alloc) == 0 {
		return fmt.Errorf("no allocs sent in update")
	}

LOCKED:
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if n.updatesLock.TryLock() {
				break LOCKED
			}
		}
	}

	n.updates = append(n.updates, req.Alloc...)

	for _, alloc := range req.Alloc {
		n.updatedClients[alloc.NodeID] = struct{}{}
		//		n.updatesMap[alloc.ID] = alloc
	}

	// This code is lifted from the Node.UpdateAlloc handler
	future := n.updateFuture
	if future == nil {
		future = structs.NewBatchFuture()
		n.updateFuture = future
		n.updateTimer = time.AfterFunc(n.serverBatchUpdateInterval, func() {
			n.updatesLock.Lock()
			updates := n.updates
			//updatesMap := n.updatesMap
			updatedClients := n.updatedClients
			future := n.updateFuture

			// Assume future update patterns will be similar to
			// current batch and set cap appropriately to avoid
			// slice resizing.
			n.updates = make([]*structs.Allocation, 0, len(updates))
			//n.updatesMap = make(map[string]*structs.Allocation, len(updatesMap))
			n.updatedClients = map[string]struct{}{}

			n.updateFuture = nil
			n.updateTimer = nil
			if !n.serverLockOnWrite {
				n.updatesLock.Unlock()
			}

			// Perform the batch update
			n.batchUpdate(future, updates, len(updatedClients))

			if n.serverLockOnWrite {
				n.updatesLock.Unlock()
			}

		})
	}

	reply.CurrentBatchSize = len(n.updatesMap)
	reply.CurrentBatchNumClients = len(n.updatedClients)
	reply.LastBatchSize = n.updatesLastBatchSize
	reply.LastBatchNumClients = n.updatesLastBatchNumClients
	reply.LastBatchTimeToWrite = n.updatesLastBatchTimeToWrite

	n.updatesLock.Unlock()

	select {
	case <-ctx.Done():
		return nil
	case <-future.WaitCh():
		if err := future.Error(); err != nil {
			return err
		}
	}

	reply.Index = future.Index()
	return nil
}

func (n *throttleTestNodeHandler) batchUpdate(future *structs.BatchFuture, updates []*structs.Allocation, numClients int) {
	if len(updates) == 0 {
		return
	}
	now := time.Now()

	// note: need to enforce only one writer here
	n.storeIndex++
	n.storeLock.Lock()
	for _, update := range updates {
		n.store[update.ID] = update
	}
	n.storeLock.Unlock()

	// simulate the cost of writes
	time.Sleep(n.serverPerWrite * time.Duration(len(updates)))
	//time.Sleep(helper.RandomStagger(n.serverBaseWriteLatency))
	time.Sleep(n.serverBaseWriteLatency)

	n.updatesLastBatchSize = len(updates)
	n.updatesLastBatchNumClients = numClients

	elapsed := time.Since(now)
	n.updatesLastBatchTimeToWrite = elapsed
	n.history = append(n.history, &batchHistory{
		lastBatchSize:        n.updatesLastBatchSize,
		lastBatchNumClients:  n.updatesLastBatchNumClients,
		lastBatchTimeToWrite: elapsed,
	})

	future.Respond(n.storeIndex, nil)
}

type throttleTestClient struct {
	nodeID             string
	syncInterval       time.Duration
	allocEventInterval time.Duration
	allocs             []*structs.Allocation
	allocUpdates       chan *structs.Allocation

	dynamicBackoffLast       bool
	backoffThresholdLast     int
	dynamicBackoffCurrent    bool
	backoffThresholdCurrent  int
	dynamicBackoffMultiplier float64
	clientNoBackoff          bool
	alwaysBackoff            bool

	responses     []*throttleTestResponse
	responsesLock sync.RWMutex
	srv           *throttleTestNodeHandler
}

func newThrottleTestClient(srv *throttleTestNodeHandler, cfg *throttleTestConfig) *throttleTestClient {
	c := &throttleTestClient{
		nodeID:             uuid.Generate(),
		syncInterval:       cfg.clientBatchUpdateInterval,
		allocEventInterval: cfg.clientAllocEventsBaseInterval,
		allocUpdates:       make(chan *structs.Allocation, 256),

		backoffThresholdLast:     cfg.clientBackoffThresholdLast,
		dynamicBackoffLast:       cfg.clientDynamicBackoffLast,
		dynamicBackoffCurrent:    cfg.clientDynamicBackoffCurrent,
		backoffThresholdCurrent:  cfg.clientBackoffThresholdCurrent,
		dynamicBackoffMultiplier: cfg.clientDynamicBackoffMultiplier,
		clientNoBackoff:          cfg.clientNoBackoff,
		responses:                []*throttleTestResponse{},
		srv:                      srv,
	}
	if c.dynamicBackoffMultiplier == 0 {
		c.dynamicBackoffMultiplier = 1.0
	}
	for i := 0; i < cfg.allocsPerClient; i++ {
		// build a bare minimum allocation for demonstration purposes
		c.allocs = append(c.allocs, &structs.Allocation{
			ID:           uuid.Generate(),
			NodeID:       c.nodeID,
			ClientStatus: structs.AllocClientStatusRunning,
			TaskStates:   map[string]*structs.TaskState{},
		})
	}
	return c
}

func (c *throttleTestClient) run(ctx context.Context) {
	go c.allocSync(ctx)
	for _, alloc := range c.allocs {
		go c.runAlloc(ctx, alloc)
	}
}

func (c *throttleTestClient) runAlloc(ctx context.Context, alloc *structs.Allocation) {
	//interval := helper.RandomStagger(c.allocEventInterval)
	interval := c.allocEventInterval
	eventTicker := time.NewTicker(interval)
	defer eventTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-eventTicker.C:
			c.allocUpdates <- alloc // ignoring dedupe for now
			//interval = helper.RandomStagger(c.allocEventInterval) + 1
			eventTicker.Reset(interval)
		}
	}
}

func (c *throttleTestClient) allocSync(ctx context.Context) {
	// sleep at the start to help randomize the client updates
	waitInterval := helper.RandomStagger(c.syncInterval)
	time.Sleep(waitInterval)

	syncInterval := c.syncInterval
	syncTicker := time.NewTicker(syncInterval)
	defer syncTicker.Stop()
	updates := make(map[string]*structs.Allocation)

	previous := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case alloc := <-c.allocUpdates:
			updates[alloc.ID] = alloc
		case <-syncTicker.C:
			if len(updates) == 0 {
				continue
			}

			args := &structs.AllocUpdateRequest{Alloc: maps.Values(updates)}
			var resp throttleTestResponse

			now := time.Now()
			err := c.srv.NodeUpdateAlloc(ctx, args, &resp)
			if err != nil { // don't clear, but backoff
				syncInterval = c.syncInterval + helper.RandomStagger(c.syncInterval)
				syncTicker.Reset(syncInterval)
				continue
			}
			resp.elapsedClientWaitTime = time.Since(previous)
			resp.elapsedClientRPCTime = time.Since(now)
			previous = time.Now()

			c.responsesLock.Lock()
			c.responses = append(c.responses, &resp)
			c.responsesLock.Unlock()
			updates = make(map[string]*structs.Allocation, len(updates))

			syncInterval = c.getInterval(&resp)
			syncTicker.Reset(syncInterval)
		}
	}
}

func (c *throttleTestClient) getInterval(resp *throttleTestResponse) time.Duration {
	syncInterval := c.syncInterval
	if c.clientNoBackoff {
		return syncInterval
	}

	backOffBase := float64(0)

	switch {
	case c.backoffThresholdLast > 0 && resp.LastBatchSize > c.backoffThresholdLast:
		if c.dynamicBackoffLast {
			backOffBase = float64(resp.LastBatchSize - c.backoffThresholdLast)
		} else {
			backOffBase = float64(helper.RandomStagger(c.syncInterval).Milliseconds())
		}

	case c.backoffThresholdCurrent > 0 && resp.CurrentBatchSize > c.backoffThresholdCurrent:
		if c.dynamicBackoffCurrent {
			backOffBase = float64(resp.CurrentBatchSize - c.backoffThresholdCurrent)
		} else {
			backOffBase = float64(helper.RandomStagger(c.syncInterval).Milliseconds())
		}
	case c.alwaysBackoff:
		if c.dynamicBackoffLast {
			backOffBase = float64(resp.LastBatchSize)
		} else if c.dynamicBackoffCurrent {
			backOffBase = float64(resp.CurrentBatchSize)
		} else {
			backOffBase = float64(helper.RandomStagger(c.syncInterval).Milliseconds())
		}
	}

	syncInterval = c.syncInterval + time.Duration(int(c.dynamicBackoffMultiplier*backOffBase))*time.Millisecond
	return syncInterval
}

type throttleTestRequest struct {
	Allocs []*structs.Allocation

	structs.WriteRequest
}

type throttleTestResponse struct {
	LastBatchSize          int
	LastBatchNumClients    int
	LastBatchTimeToWrite   time.Duration
	CurrentBatchSize       int
	CurrentBatchNumClients int

	elapsedClientRPCTime  time.Duration
	elapsedClientWaitTime time.Duration

	structs.WriteMeta
}

func batchStats(values []float64) (float64, float64, float64) {
	sumSq := float64(0)
	sum := float64(0)
	max := float64(0)

	for _, val := range values {
		sum += val
		if val > max {
			max = val
		}
		sumSq += val * val
	}
	mean := sum / float64(len(values))
	stddev := math.Sqrt((sumSq/float64(len(values)) - (mean * mean)))
	return mean, stddev, max
}

func TestAllocSyncThrottling(t *testing.T) {

	testWindow := time.Duration(10 * time.Second)
	numClients := 1000
	allocsPerClient := 100
	serverBatchUpdateInterval := time.Millisecond * 50
	clientBatchUpdateInterval := time.Millisecond * 200
	clientAllocEventsBaseInterval := time.Millisecond * 50
	perWrite := time.Microsecond * 100
	batchLatency := time.Millisecond * 5

	testCfgs := []*throttleTestConfig{

		{
			name:                           "default",
			clientNoBackoff:                true,
			clientDynamicBackoffMultiplier: 1.0,
		},
		{
			name:                           "backoff_server_lock",
			serverLockOnWrite:              true,
			clientDynamicBackoffMultiplier: 1.0,
		},

		{
			name:                           "backoff_fully_dynamic_last",
			clientDynamicBackoffLast:       true,
			clientDynamicBackoffMultiplier: 1.0,
			alwaysBackoff:                  true,
		},
		{
			name:                           "backoff_fully_dynamic_current",
			clientDynamicBackoffCurrent:    true,
			clientDynamicBackoffMultiplier: 1.0,
			alwaysBackoff:                  true,
		},

		{
			name:                           "backoff_5000_last_random",
			clientBackoffThresholdLast:     5000,
			clientDynamicBackoffMultiplier: 1.0,
		},

		{
			name:                           "backoff_5000_current_random",
			clientBackoffThresholdCurrent:  5000,
			clientDynamicBackoffMultiplier: 1.0,
		},

		{
			name:                           "backoff_5000_dynamic_last",
			clientBackoffThresholdLast:     5000,
			clientDynamicBackoffLast:       true,
			clientDynamicBackoffMultiplier: 1.0,
		},
		{
			name:                           "backoff_5000_dynamic_current",
			clientBackoffThresholdCurrent:  5000,
			clientDynamicBackoffCurrent:    true,
			clientDynamicBackoffMultiplier: 1.0,
		},

		{
			name:                           "backoff_5000_dynamic_last_half",
			clientBackoffThresholdLast:     5000,
			clientDynamicBackoffLast:       true,
			clientDynamicBackoffMultiplier: 0.5,
		},
		{
			name:                           "backoff_5000_dynamic_current_half",
			clientBackoffThresholdCurrent:  5000,
			clientDynamicBackoffCurrent:    true,
			clientDynamicBackoffMultiplier: 0.5,
		},

		{
			name:                           "backoff_1000_last_random",
			clientBackoffThresholdLast:     1000,
			clientDynamicBackoffMultiplier: 1.0,
		},

		{
			name:                           "backoff_1000_current_random",
			clientBackoffThresholdCurrent:  1000,
			clientDynamicBackoffMultiplier: 1.0,
		},

		{
			name:                           "backoff_1000_dynamic_last",
			clientBackoffThresholdLast:     1000,
			clientDynamicBackoffLast:       true,
			clientDynamicBackoffMultiplier: 1.0,
		},
		{
			name:                           "backoff_1000_dynamic_current",
			clientBackoffThresholdCurrent:  1000,
			clientDynamicBackoffCurrent:    true,
			clientDynamicBackoffMultiplier: 1.0,
		},

		{
			name:                           "backoff_1000_dynamic_last_half",
			clientBackoffThresholdLast:     1000,
			clientDynamicBackoffLast:       true,
			clientDynamicBackoffMultiplier: 0.5,
		},
		{
			name:                           "backoff_1000_dynamic_current_half",
			clientBackoffThresholdCurrent:  1000,
			clientDynamicBackoffCurrent:    true,
			clientDynamicBackoffMultiplier: 0.5,
		},

		{
			name:                           "backoff_500_last_random",
			clientBackoffThresholdLast:     500,
			clientDynamicBackoffMultiplier: 1.0,
		},
		{
			name:                           "backoff_500_current_random",
			clientBackoffThresholdCurrent:  500,
			clientDynamicBackoffMultiplier: 1.0,
		},
		{
			name:                           "backoff_500_dynamic_last",
			clientBackoffThresholdLast:     500,
			clientDynamicBackoffLast:       true,
			clientDynamicBackoffMultiplier: 1.0,
		},
		{
			name:                           "backoff_500_dynamic_current",
			clientBackoffThresholdCurrent:  500,
			clientDynamicBackoffCurrent:    true,
			clientDynamicBackoffMultiplier: 1.0,
		},
	}

	results := []string{
		"Backoff Threshold|Backoff Metric|Backoff Qty|Mult|# Batches|Updates/Batch|Clients/Batch|Time/Batch (ms)|# Responses|RPC Latency (ms)|Client Wait Time (ms)",
		"---|---|---|---|---|---|---|---|---|---|---",
	}

	for _, cfg := range testCfgs {

		t.Run(cfg.name, func(t *testing.T) {
			fmt.Printf(" [%s] starting\n", time.Now().Format(time.RFC3339))
			cfg.numClients = numClients
			cfg.allocsPerClient = allocsPerClient
			cfg.serverBatchUpdateInterval = serverBatchUpdateInterval
			cfg.clientBatchUpdateInterval = clientBatchUpdateInterval
			cfg.clientAllocEventsBaseInterval = clientAllocEventsBaseInterval
			cfg.serverPerWrite = perWrite
			cfg.serverBaseWriteLatency = batchLatency
			srv := newThrottleTestNodeHandler(cfg)
			fmt.Printf(" [%s] server created\n", time.Now().Format(time.RFC3339))

			clients := []*throttleTestClient{}
			for i := 0; i < cfg.numClients; i++ {
				clients = append(clients, newThrottleTestClient(srv, cfg))
			}
			fmt.Printf(" [%s] clients created\n", time.Now().Format(time.RFC3339))

			ctx, cancel := context.WithCancel(context.TODO())
			defer cancel()
			for _, client := range clients {
				client.run(ctx)
			}
			fmt.Printf(" [%s] clients running\n", time.Now().Format(time.RFC3339))
			time.AfterFunc(testWindow, cancel)
			<-ctx.Done()
			fmt.Printf(" [%s] stopped test, waiting 1s to wrap up...\n", time.Now().Format(time.RFC3339))
			time.Sleep(time.Second)

			srv.updatesLock.Lock()
			defer srv.updatesLock.Unlock()
			result := resultFromHistory(cfg, srv.history, clients)
			results = append(results, result)
			srv.history = []*batchHistory{}
			fmt.Printf(" [%s] results captured\n", time.Now().Format(time.RFC3339))
		})
	}

	fmt.Printf("# Clients: %d\n# Allocs: %v\nServer Batch Interval: %v\nClient Batch Interval: %v\nAlloc Event Interval: %v\n",
		numClients,
		allocsPerClient,
		serverBatchUpdateInterval,
		clientBatchUpdateInterval,
		clientAllocEventsBaseInterval)

	columnizeCfg := columnize.DefaultConfig()
	columnizeCfg.Glue = " | "
	columnizeCfg.Prefix = "| "
	fmt.Println(columnize.Format(results, columnizeCfg))
}

func resultFromHistory(cfg *throttleTestConfig, history []*batchHistory, clients []*throttleTestClient) string {

	batchSizes := helper.ConvertSlice(history, func(b *batchHistory) float64 {
		return float64(b.lastBatchSize)
	})
	batchClients := helper.ConvertSlice(history, func(b *batchHistory) float64 {
		return float64(b.lastBatchNumClients)
	})
	times := helper.ConvertSlice(history, func(b *batchHistory) float64 {
		return float64(b.lastBatchTimeToWrite.Milliseconds())
	})

	meanBatchSize, stdBatchSize, maxBatchSize := batchStats(batchSizes)
	meanBatchClients, stdBatchClients, maxBatchClients := batchStats(batchClients)
	meanBatchTime, stdBatchTimes, maxBatchTime := batchStats(times)

	clientResponses := 0
	clientRPCLatencies := []float64{}
	clientWaitLatencies := []float64{}

	for _, client := range clients {
		client.responsesLock.Lock()
		defer client.responsesLock.Unlock()
		clientResponses += len(client.responses)
		for _, resp := range client.responses {
			clientRPCLatencies = append(clientRPCLatencies, float64(resp.elapsedClientRPCTime.Milliseconds()))
			clientWaitLatencies = append(clientWaitLatencies, float64(resp.elapsedClientWaitTime.Milliseconds()))

		}
		client.responses = []*throttleTestResponse{}
	}

	meanClientRPCLatency, stdClientRPCLatency, maxClientRPCLatency := batchStats(clientRPCLatencies)

	meanClientWaitLatency, stdClientWaitLatency, maxClientWaitLatency := batchStats(clientWaitLatencies)

	backoffMetric := "none"
	backoffQty := "none"
	threshold := "0"
	if cfg.clientBackoffThresholdCurrent > 0 {
		threshold = string(cfg.clientBackoffThresholdCurrent)
		backoffMetric = "current"
		backoffQty = "random"
		if cfg.clientDynamicBackoffCurrent {
			backoffQty = "dynamic"
		}

	}
	if cfg.clientBackoffThresholdLast > 0 {
		threshold = string(cfg.clientBackoffThresholdLast)
		backoffMetric = "last"
		backoffQty = "random"
		if cfg.clientDynamicBackoffLast {
			backoffQty = "dynamic"
		}
	}
	if cfg.serverLockOnWrite {
		backoffMetric = "server lock"
		backoffQty = "n/a"
	}
	if cfg.alwaysBackoff {
		threshold = "always"
		backoffQty = "dynamic"
		if cfg.clientDynamicBackoffLast {
			backoffMetric = "last"
		}
		if cfg.clientDynamicBackoffCurrent {
			backoffMetric = "current"
		}
	}

	return fmt.Sprintf("%s|%s|%s|%.1f|%d|%.0f ± %.0f (max %.0f)|%.0f ± %.0f (max %.0f)|%.0f ± %.0f (max %.0f)|%d|%.0f ± %.0f (max %.0f)|%.0f ± %.0f (max %.0f)",
		threshold,
		backoffMetric,
		backoffQty,
		cfg.clientDynamicBackoffMultiplier,
		len(history),
		meanBatchSize,
		stdBatchSize,
		maxBatchSize,
		meanBatchClients,
		stdBatchClients,
		maxBatchClients,
		meanBatchTime,
		stdBatchTimes,
		maxBatchTime,
		clientResponses,
		meanClientRPCLatency,
		stdClientRPCLatency,
		maxClientRPCLatency,
		meanClientWaitLatency,
		stdClientWaitLatency,
		maxClientWaitLatency,
	)

}
