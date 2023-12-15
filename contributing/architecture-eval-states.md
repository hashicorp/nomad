# Architecture: Evaluation Status

The [Scheduling in Nomad][] internals documentation covers the path that an
evaluation takes through the leader, worker, and plan applier. But it doesn't
cover in any detail the various `Evaluation.Status` values, or where the
`PreviousEval`, `NextEval`, or `BlockedEval` ID pointers are set.

The state diagram below describes the transitions between `Status` values as
solid arrows. The dashed arrows represent when a new evaluation is created. The
parenthetical labels on those arrows are the `TriggeredBy` field for the new
evaluation.

The status values are:

* `pending` evaluations either are queued to be scheduled, are still being
  processed in the scheduler, or are being applied by the plan applier and not
  yet acknowledged.
* `failed` evaluations have failed to be applied by the plan applier (or are
  somehow invalid in the scheduler; this is always a bug)
* `blocked` evaluations are created when an eval has failed too many attempts to
  have its plan applied by the leader, or when a plan can only be partially
  applied and there are still more allocations to create.
* `complete` means the plan was applied successfully (at least partially).
* `canceled` means the evaluation was superseded by state changes like a new
  version of the job.


```mermaid
flowchart LR

    event((Cluster\nEvent))

    pending([pending])
    blocked([blocked])
    complete([complete])
    failed([failed])
    canceled([canceled])

    %% style classes
    classDef status fill:#d5f6ea,stroke-width:4px,stroke:#1d9467
    classDef other fill:#d5f6ea,stroke:#1d9467
    class event other;
    class pending,blocked,complete,failed,canceled status;

    event -. "job-register
      job-deregister
      periodic-job
      node-update
      node-drain
      alloc-stop
      scheduled
      alloc-failure
      job-scaling" .-> pending

    pending -. "new eval\n(rolling-update)" .-> pending
    pending -. "new eval\n(preemption)" .-> pending

    pending -. "new eval\n(max-plan-attempts)" .-> blocked
    pending -- if plan submitted --> complete
    pending -- if invalid --> failed
    pending -- if no-op --> canceled

    failed -- if retried --> blocked
    failed -- if retried --> complete

    blocked -- if no-op --> canceled
    blocked -- if plan submitted --> complete

    complete -. "new eval\n(deployment-watcher)" .-> pending
    complete -. "new eval\n(queued-allocs)" .-> blocked

    failed -. "new eval\n(failed-follow-up)" .-> pending
```

But it's hard to get a full picture of the evaluation lifecycle purely from the
`Status` fields, because evaluations have several "quasi-statuses" which aren't
represented as explicit statuses in the state store:

* `scheduling` is the status where an eval is being processed by the scheduler
  worker.
* `applying` is the status where the resulting plan for the eval is being
  applied in the plan applier on the leader.
* `delayed` is an enqueued eval that will be dequeued some time in the future.
* `deleted` is when an eval is removed from the state store entirely.

By adding these statuses to the diagram (the dashed nodes), you can see where
the same `Status` transition might result in different `PreviousEval`,
`NextEval`, or `BlockedEval` set. You can also see where the "chain" of
evaluations is broken when new evals are created for preemptions or by the
deployment watcher.


```mermaid
flowchart LR

    event((Cluster\nEvent))

    %% statuss
    pending([pending])
    blocked([blocked])
    complete([complete])
    failed([failed])
    canceled([canceled])

    %% quasi-statuss
    deleted([deleted])
    delayed([delayed])
    scheduling([scheduling])
    applying([applying])

    %% style classes
    classDef status fill:#d5f6ea,stroke-width:4px,stroke:#1d9467
    classDef quasistatus fill:#d5f6ea,stroke-dasharray: 5 5,stroke:#1d9467
    classDef other fill:#d5f6ea,stroke:#1d9467

    class event other;
    class pending,blocked,complete,failed,canceled status;
    class deleted,delayed,scheduling,applying quasistatus;

    event -- "job-register
      job-deregister
      periodic-job
      node-update
      node-drain
      alloc-stop
      scheduled
      alloc-failure
      job-scaling" --> pending

    pending -- dequeued --> scheduling
    pending -- if delayed --> delayed
    delayed -- dequeued --> scheduling

    scheduling -. "not all allocs placed
      new eval created by scheduler
      trigger queued-allocs
      new has .PreviousEval = old.ID
      old has .BlockedEval = new.ID" .-> blocked

    scheduling -. "failed to plan
      new eval created by scheduler
      trigger: max-plan-attempts
      new has .PreviousEval = old.ID
      old has .BlockedEval = new.ID" .-> blocked

    scheduling -- "not all allocs placed
      reuse already-blocked eval" --> blocked

    blocked -- "unblocked by
      external state changes" --> scheduling

    scheduling -- allocs placed --> complete

    scheduling -- "wrong eval type or
      max retries exceeded
      on plan submit" --> failed

    scheduling -- "canceled by
      job update/stop" --> canceled

    failed -- retry --> scheduling

    scheduling -. "new eval from rolling update (system jobs)
      created by scheduler
      trigger: rolling-update
      new has .PreviousEval = old.ID
      old has .NextEval = new.ID" .-> pending

    scheduling -- submit --> applying
    applying -- failed --> scheduling

    applying -. "new eval for preempted allocs
      created by plan applier
      trigger: preemption
      new has .PreviousEval = unset!
      old has .BlockedEval = unset!" .-> pending

    complete -. "new eval from deployments (service jobs)
      created by deploymentwatcher
      trigger: deployment-watcher
      new has .PreviousEval = unset!
      old has .NextEval = unset!" .-> pending

    failed -- "new eval
      trigger: failed-follow-up
      new has .PreviousEval = old.ID
      old has .NextEval = new.ID" --> pending

    pending -- "undeliverable evals
      reaped by leader" --> failed

    blocked -- "duplicate blocked evals
      reaped by leader" --> canceled

    canceled -- garbage\ncollection --> deleted
    failed -- garbage\ncollection --> deleted
    complete -- garbage\ncollection --> deleted
```


[Scheduling in Nomad]: https://developer.hashicorp.com/nomad/docs/internals/scheduling/scheduling
