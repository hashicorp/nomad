# Architecture: Nomad State Store

Nomad server state is an in-memory state store backed by raft. All writes to
state are serialized into message pack and written as raft logs. The raft logs
are replicated from the leader to the followers. Once each follower has
persisted the log entry and applied the entry to its in-memory state ("FSM"),
the leader considers the write committed.

This architecture has a few implications:

* The `fsm.Apply` functions must be deterministic over their inputs for a given
  state. You can never generate random IDs or assign wall-clock timestamps in
  the state store. These values must be provided as parameters from the RPC
  handler.

    ```go
    # Incorrect: generating a timestamp in the state store is not deterministic.
    func (s *StateStore) UpsertObject(...) {
        # ...
        obj.CreateTime = time.Now()
        # ...
    }

    # Correct: non-deterministic values should be passed as inputs:
    func (s *StateStore) UpsertObject(..., timestamp time.Time) {
        # ...
        obj.CreateTime = timestamp
        # ...
    }
    ```

* Every object you read from the state store must be copied before it can be
  mutated, because mutating the object modifies it outside the raft
  workflow. The result can be servers having inconsistent state, transactions
  breaking, or even server panics.

    ```go
    # Incorrect: job is mutated without copying.
    job, err := state.JobByID(ws, namespace, id)
    job.Status = structs.JobStatusRunning

    # Correct: only the job copy is mutated.
    job, err := state.JobByID(ws, namespace, id)
    updateJob := job.Copy()
    updateJob.Status = structs.JobStatusRunning
    ```

Adding new objects to the state store should be done as part of adding new RPC
endpoints. See the [RPC Endpoint Checklist][].

```mermaid
flowchart TD

    %% entities

    ext(("API\nclient"))
    any("Any node
      (client or server)")
    follower(Follower)

    rpcLeader("RPC handler (on leader)")

    writes("writes go thru raft
        raftApply(MessageType, entry) in nomad/rpc.go
        structs.MessageType in nomad/structs/structs.go
        go generate ./... for nomad/msgtypes.go")
    click writes href "https://github.com/hashicorp/nomad/tree/main/nomad" _blank

    reads("reads go directly to state store
        Typical state_store.go funcs to implement:

        state.GetMyThingByID
        state.GetMyThingByPrefix
        state.ListMyThing
        state.UpsertMyThing
        state.DeleteMyThing")
    click writes href "https://github.com/hashicorp/nomad/tree/main/nomad/state" _blank

    raft("hashicorp/raft")

    bolt("boltdb")

    fsm("Application-specific
      Finite State Machine (FSM)
      (aka State Store)")
    click writes href "https://github.com/hashicorp/nomad/tree/main/nomad/fsm.go" _blank

    memdb("hashicorp/go-memdb")

    %% style classes
    classDef leader fill:#d5f6ea,stroke-width:4px,stroke:#1d9467
    classDef other fill:#d5f6ea,stroke:#1d9467
    class any,follower other;
    class rpcLeader,raft,bolt,fsm,memdb leader;

    %% flows

    ext -- HTTP request --> any

    any -- "RPC request
      to connected server
      (follower or leader)" --> follower

    follower -- "(1) srv.Forward (to leader)" --> rpcLeader

    raft -- "(3) replicate to a
      quorum of followers
      wait on their fsm.Apply" --> follower

    rpcLeader --> reads
    reads --> memdb

    rpcLeader --> writes
    writes -- "(2)" --> raft

    raft -- "(4) write log to disk" --> bolt
    raft -- "(5) fsm.Apply
      nomad/fsm.go" --> fsm

    fsm -- "(6) txn.Insert" --> memdb

    bolt <-- "Snapshot Persist: nomad/fsm.go
    Snapshot Restore: nomad/fsm.go" --> memdb


    %% notes

    note1("Typical structs to implement
        for RPC handlers:

        structs.MyThing
          .Diff()
          .Copy()
          .Merge()
        structs.MyThingUpsertRequest
        structs.MyThingUpsertResponse
        structs.MyThingGetRequest
        structs.MyThingGetResponse
        structs.MyThingListRequest
        structs.MyThingListResponse
        structs.MyThingDeleteRequest
        structs.MyThingDeleteResponse

        Don't forget to register your new RPC
        in nomad/server.go!")

    note1 -.- rpcLeader
```


[RPC Endpoint Checklist]: https://github.com/hashicorp/nomad/blob/main/contributing/checklist-rpc-endpoint.md
