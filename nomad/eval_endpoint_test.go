package nomad

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestEvalEndpoint_GetEval(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mockEval()
	s1.fsm.State().UpsertEval(1000, eval1)

	// Lookup the eval
	get := &structs.EvalSpecificRequest{
		EvalID:       eval1.ID,
		WriteRequest: structs.WriteRequest{Region: "region1"},
	}
	var resp structs.SingleEvalResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.GetEval", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp.Index, 1000)
	}

	if !reflect.DeepEqual(eval1, resp.Eval) {
		t.Fatalf("bad: %#v %#v", eval1, resp.Eval)
	}

	// Lookup non-existing node
	get.EvalID = generateUUID()
	if err := msgpackrpc.CallWithCodec(codec, "Eval.GetEval", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp.Index, 1000)
	}
	if resp.Eval != nil {
		t.Fatalf("unexpected eval")
	}
}

func TestEvalEndpoint_Dequeue(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mockEval()
	s1.evalBroker.Enqueue(eval1)

	// Dequeue the eval
	get := &structs.EvalDequeueRequest{
		Schedulers:   defaultSched,
		WriteRequest: structs.WriteRequest{Region: "region1"},
	}
	var resp structs.SingleEvalResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Dequeue", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(eval1, resp.Eval) {
		t.Fatalf("bad: %#v %#v", eval1, resp.Eval)
	}

	// Ensure outstanding
	if !s1.evalBroker.Outstanding(eval1.ID) {
		t.Fatalf("should be outstanding")
	}
}

func TestEvalEndpoint_Ack(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)

	testutil.WaitForResult(func() (bool, error) {
		return s1.evalBroker.Enabled(), nil
	}, func(err error) {
		t.Fatalf("should enable eval broker")
	})

	// Create the register request
	eval1 := mockEval()
	s1.evalBroker.Enqueue(eval1)
	out, err := s1.evalBroker.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("missing eval")
	}

	// Ack the eval
	get := &structs.EvalSpecificRequest{
		EvalID:       out.ID,
		WriteRequest: structs.WriteRequest{Region: "region1"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Ack", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure outstanding
	if s1.evalBroker.Outstanding(eval1.ID) {
		t.Fatalf("should not be outstanding")
	}
}

func TestEvalEndpoint_Nack(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)

	testutil.WaitForResult(func() (bool, error) {
		return s1.evalBroker.Enabled(), nil
	}, func(err error) {
		t.Fatalf("should enable eval broker")
	})

	// Create the register request
	eval1 := mockEval()
	s1.evalBroker.Enqueue(eval1)
	out, _ := s1.evalBroker.Dequeue(defaultSched, time.Second)
	if out == nil {
		t.Fatalf("missing eval")
	}

	// Ack the eval
	get := &structs.EvalSpecificRequest{
		EvalID:       out.ID,
		WriteRequest: structs.WriteRequest{Region: "region1"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Nack", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure outstanding
	if s1.evalBroker.Outstanding(eval1.ID) {
		t.Fatalf("should not be outstanding")
	}

	// Should get it back
	out2, _ := s1.evalBroker.Dequeue(defaultSched, time.Second)
	if out2 != out {
		t.Fatalf("nack failed")
	}
}
