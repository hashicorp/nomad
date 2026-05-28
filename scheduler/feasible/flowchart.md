# Feasible Package Flow

This package is easiest to understand in two phases:

- stack construction: build a chain of iterators and checkers
- selection: push one task group through that chain until one node wins

## 1. Generic stack construction

```mermaid
flowchart TD
    A[Base node iterator]
    A --> B[FeasibilityWrapper]
    B --> B1[job checks]
    B1 --> B1a[DependencyChecker]
    B1 --> B1b[ConstraintChecker]
    B --> B2[task group checks]
    B2 --> B2a[DriverChecker]
    B2 --> B2b[ConstraintChecker]
    B2 --> B2c[DeviceChecker]
    B2 --> B2d[NetworkChecker]
    B2 --> B2e[SecretsProviderChecker]
    B --> B3[availability checks]
    B3 --> B3a[HostVolumeChecker]
    B3 --> B3b[CSIVolumeChecker]
    B --> C[DistinctHostsIterator]
    C --> D[DistinctPropertyIterator]
    D --> E[QuotaIterator]
    E --> F[FeasibleRankIterator]
    F --> G[BinPackIterator]
    G --> H[JobAntiAffinityIterator]
    H --> I[NodeReschedulingPenaltyIterator]
    I --> J[NodeAffinityIterator]
    J --> K[SpreadIterator]
    K --> L[PreemptionScoringIterator]
    L --> M[ScoreNormalizationIterator]
    M --> N[LimitIterator]
    N --> O[MaxScoreIterator]
```

This is the main idea in [scheduler/feasible/stack.go](/Users/juanita.delacuestamorales/go/src/github.com/hashicorp/nomad/scheduler/feasible/stack.go):

- everything before `FeasibleRankIterator` is still filtering nodes out
- everything after `FeasibleRankIterator` is scoring or selecting among feasible nodes
- `MaxScoreIterator` is what finally chooses the winning node

## 2. What happens during Select

```mermaid
flowchart TD
    A[Select called with task group and options] --> B{Preferred nodes present}
    B -->|yes| C[Try only preferred nodes first]
    C --> D{Found a feasible ranked node}
    D -->|yes| Z[Return that node]
    D -->|no| E[Restore full node set]
    B -->|no| F[Reset scores and eval context]
    E --> F

    F --> G[Collect task group requirements]
    G --> H[drivers]
    G --> I[constraints]
    G --> J[devices]
    G --> K[networks]
    G --> L[secrets]
    G --> M[volumes]
    G --> N[affinities and spread]

    H --> O[Update iterators with current task group]
    I --> O
    J --> O
    K --> O
    L --> O
    M --> O
    N --> O

    O --> P[Begin iterating candidate nodes]
    P --> Q{Computed class already known}
    Q -->|ineligible| P
    Q -->|eligible or unknown| R[Run job level feasibility]

    R --> R1{Dependencies ready}
    R1 -->|no| P
    R1 -->|yes| R2{Job constraints pass}
    R2 -->|no| P
    R2 -->|yes| S[Run task group feasibility]

    S --> S1{Drivers constraints devices network secrets pass}
    S1 -->|no| P
    S1 -->|yes| T[Run availability checks]

    T --> T1{Host volumes and CSI volumes available}
    T1 -->|no| P
    T1 -->|yes| U[Pass node into ranking pipeline]

    U --> V[Apply distinct host and distinct property rules]
    V --> W[Apply quota check]
    W --> X[Score node]
    X --> X1[binpack]
    X --> X2[job anti affinity]
    X --> X3[reschedule penalty]
    X --> X4[node affinity]
    X --> X5[spread]
    X --> X6[preemption scoring]
    X --> Y[Normalize scores then limit search]
    Y --> Z[Return highest scoring node]
```

## How to read the mechanism

1. `SetNodes` chooses the starting population of nodes. In the generic stack it also shuffles them and sets a search limit.
2. `SetJob` pushes job-wide state into the iterators: job constraints, dependencies, distinctness, affinity, spread, quota context, and namespace or job IDs for volume checks.
3. `Select` pushes task-group-specific state into the same chain: drivers, constraints, devices, volumes, network, secrets, and any scoring context.
4. The feasibility wrapper is the hard gate. A node that fails there never reaches ranking.
5. The first important split is filter versus score. Filters answer "can this node run the task group at all". Scorers answer "which feasible node is best".
6. Dependencies are part of the job-level filter stage. If they are not ready, the node is rejected before any later ranking matters.
7. Distinct host, distinct property, and quota still behave like feasibility filters even though they are implemented as iterators later in the chain.
8. The final answer comes from the max-score step after normalization and limit logic.

## Files to map back to code

- [scheduler/feasible/stack.go](/Users/juanita.delacuestamorales/go/src/github.com/hashicorp/nomad/scheduler/feasible/stack.go) builds the iterator chain and drives `Select`.
- [scheduler/feasible/feasible.go](/Users/juanita.delacuestamorales/go/src/github.com/hashicorp/nomad/scheduler/feasible/feasible.go) contains the concrete feasibility checks.
- [scheduler/feasible/dependencies.go](/Users/juanita.delacuestamorales/go/src/github.com/hashicorp/nomad/scheduler/feasible/dependencies.go) handles job dependency readiness.
