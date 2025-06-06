# Nomad Scheduler

This package holds the logic behind Nomad schedulers. The `Scheduler` interface
is implemented by two objects:

- `GenericScheduler` and
- `SystemScheduler`.

The `CoreScheduler` object also implements this interface, but it's use is
purely internal, the core scheduler does not schedule any user jobs.

Nomad scheduler's task is to, given an evaluation, produce a plan of placing the
desired allocations on feasibile nodes. Consult [Nomad documentation][0] for
more details.

The diagram below illustrates this process for the service and system schedulers
in more detail:

```
                                 +--------------+        +-----------+                            +-------------+      +----------+
                                 |   cluster    |        |feasibility|       +-------------+      |    score    |      |   plan   |
    Service and batch jobs:      |reconciliation|------->|   check   |------>|     fit     |----->| assignment  |----->|submission|
                                 +--------------+        +-----------+       +-------------+      +-------------+      +----------+

- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

                                 +--------------+        +-----------+                                                 +----------+
 System and sysbatch jobs:       |     node     |        |feasibility|       +-------------+                           |   plan   |
                                 |reconciliation|------->|   check   |------>|     fit     |-------------------------->|submission|
                                 +--------------+        +-----------+       +-------------+                           +----------+
```

## Reconciliation

The first step for the service and bach job scheduler is called
"reconciliation," and its logic lies in the `scheduler/reconciler` package.
There are two reconcilers: `AllocReconciler` object for service and batch jobs,
and `Node` reconciler used by system and sysbatch jobs.

Both reconciler's task is to tell the scheduler about desired allocations or
deployments to be updated or created, which should be updated destructively or
in-place, which should be stopped, and which are disconnected or are
reconnecting. The reconciler works in terms of "buckets," that is, it processes
allocations by putting them into different sets, and that's how its whole logic
is implemented.

The following vocabulary is used by the reconcilers:

- "tainted node:" a node is considered "tainted" if allocations must be migrated
off of it. These are nodes that are draining or have been drained, but also
nodes that are disconnected and should be used to calculated reconnect timeout.
The cluster reconciler commonly also refers to "untainted" allocations, i.e.,
those that do not require migration and are not on disconnected or reconnecting
nodes.

- "paused deployment:" a deployment is paused when it has an explicit `paused`
status, but also when it's pending or initializing.

- the reconciler uses the following 6 "buckets" to categorize allocation sets:

  - "migrating allocations:" allocations that are on nodes that are draining.

  - "lost allocations:" allocations that have expired or exist on lost nodes.

  - "disconnecting allocations:" allocations that are on disconnected nodes
    which haven't been considered "lost" yet, that is, they are in their reconnect
    timeout.

  - "reconnecting allocations:" allocations on nodes that have reconnected.

  - "ignored allocations:" allocations which are in a noop state, the reconciler
     will not be touching these. These are also not to be upgraded in-place,
     for updates, the reconciler uses additional "buckets" (in the `computeUpdates`
     method): "inplace" and "destructive."

  - "expiring allocations:" allocations which are not possible to reschedule, due
     to lost configurations of their disconnected clients.

### Cluster Reconciler

The following diagram illustrates the logic flow of the cluster reconciler:

```
          +---------+
          |Compute()|
          +---------+
               |
               v
        +------------+
        |create a new|          allocMatrix is created from existing
        | allocation |          allocations for a job, and is a map of
        |  matrix m  |          task groups to allocation sets.
        +------------+
               |
               v                deployments are unneeded in 3 cases:
 +---------------------------+  1. when the are already successful
 |cancelUnneededDeployments()|  2. when they are active but reference an older job
 +---------------------------+  3. when the job is marked as stopped, but the
               |                deployment is non-terminal
               v
         +-----------+          if the job is stopped, we stop
         |handle stop|          all allocations and handle the
         +-----------+          lost allocations.
               |
               v
  +-------------------------+   sets deploymentPaused and
  |computeDeploymentPaused()|   deploymentFailed fields on the
  +-------------------------+   reconciler.
               |
               |
               |                for every task group, this method
               v                calls computeGroup which returns
+----------------------------+  "true" if deployment is complete
|computeDeploymentComplete(m)|  for the task group.
+----------------------------+  computeDeploymentComplete itself
               |                returns a boolean.
               |
               |              +------------------------------------+
               |              |    computeGroup(groupName, all     |
               +------------->|            allocations)            |
               |              +------------------------------------+
               |
               |                contains the main, and most complex part
               |                of the reconciler. it calls many helper
               |                methods:
               |                - filterOldTerminalAllocs: allocs that
               |                are terminal or from older job ver are
               |                put into "ignore" bucket
               |                - cancelUnneededCanaries
               |                - filterByTainted: results in 6 buckets
               |                mentioned in the paragraphs above:
               |                untainted, migrate, lost, disconnecting,
               |                reconnecting, ignore and expiring.
               |                - filterByRescheduleable: updates the
               |                untainted bucket and creates 2 new ones:
               |                rescheduleNow and rescheduleLater
               +-------+        - reconcileReconnecting: returns which
                       |        allocs should be marked for reconnecting
                       |        and which should be stopped
                       |        - computeStop
                       |        - computeCanaries
                       |        - computePlacements: allocs are placed if
                       |        deployment is not paused or failed, they
                       |        are not canaries (unless promoted),
                       |        previous alloc was lost
                       |        - computeReplacements
                       |        - createDeployment
                       |
                       |
                       |
                       |
                       |
                       |
                       |
                       v
+--------------------------------------------+
|computeDeploymentUpdates(deploymentComplete)|
+--------------------------------------------+
                       |
                +------+        for complete deployments, it
                |               handles multi-region case and
                v               sets the deploymentUpdates
        +---------------+
        |return a.result|
        +---------------+
```

### Node Reconciler

TODO

## Feasibility checking

Nomad uses a set of iterators to iterate over nodes and check how feasible
they are for any given allocation. The scheduler uses a `Stack` interface that
lives in `scheduler/stack.go` file in order to make placement decisions, and
feasibility iterators that live in `scheduler/feasible.go` to filter by:

- node eligibiligy,
- data center,
- and node pool.

Once nodes are filtered, the `Stack` implementations (`GenericStack` and
`SystemStack`) check for:

- drivers,
- job constraints,
- devices,
- volumes,
- networking,
- affinities,
- and quotas.

## Node reconciliation

The system scheduler also does a "reconciliation" step, but only on a
per-node basis (system jobs run on all feasible nodes), which makes it
simpler than the service reconciler which takes into account a whole cluster,
and has jobs that can run on arbitrary subset of clients. The code is in
`scheduler/scheduler_system.go` file.

Node reconciliation removes tainted nodes, updates terminal allocations to lost,
deals with disconnected nodes and computes placements.

## Finding the best fit and scoring

Applies only to service and batch jobs, since system and sysbatch jobs are
placed on all feasible nodes.

This part of scheduling sits in the `scheduler/rank.go` file. The `RankIterator`
interface, which is implemented by e.g., `SpreadIterator` and `BinPackIterator`,
captures the ranking logic in its `Next()` methods.

[0]: https://developer.hashicorp.com/nomad/docs/concepts/scheduling/scheduling
