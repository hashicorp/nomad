# Overview

The TaskGroupReconciler is drop-in replacement of the current `allocReconciler.computeGroup` method.
Its purpose is to assess the current cluster state for a given `TaskGroup` versus the desired state
and calculate what changes need to be made to get the cluster into the desired state. It considers
factors such as:

- Client node status
- Job version
- Allocation desired status
- Allocation client status
- Allocation reschedule policy
- Deployment status
- Canary status
- Disconnect configuration

## Hypothesis

The current approach to managing allocations is reminiscent of a stored procedure that tries to
manage a number of in-memory result sets. Specifically, it starts with a set of entities of
interest, in this case `Allocations`, and then progressively groups and/or filters the set. 
This approach to state management is particularly challenging because it introduces the cognitive 
overhead of understanding the state mutations that have already been applied to each set prior
at any given point in the processing chain. While this approach is necessitated in languages
inhibited by a lack of expressiveness, like SQL is, `go` is a full-featured imperative
language that is not limited by a lack of language features. It should be possible to devise
a domain model that provides the correct results in a more readable, maintainable, extensible,
and testable way.

## Requirements

- Needs to build a `reconcileResults` that the `GenericScheduler` can consume.
- Needs to be hot swappable with `allocReconciler.computeGroup`
- Should be hot swappable with current unit tests and not fail
- Needs to clearly improve readability by:
  - Encapsulating business rules with domain methods
  - Describing the end state in terms of domain methods
  - Initialization of state fields
  - Expressing state filters as domain methods and/or state field comparisons
- Needs to clearly improve maintainability by:
  - Minimizing and localizing state mutations.
  - Moving to a domain model instead of set filtering
  - Reducing the number of multiphase calculations (e.g. filterByTainted, filterByReschedulable, computeStop)
- Needs to clearly improve extensibility by:
  - Exposing explicit extension mechanisms.
  - Providing a TDD friendly interface.

## Domain Model && Workflow

Rather than adopting a paradigm based on set theory, the `TaskGroupReconciler`implements the
functionality using a domain model. The `TaskGroupReconciler` acts as the primary aggregate
root. It accepts the incoming cluster state and desired `TaskGroup` configuration. `Allocation`
instances for a `TaskGroup` have a `Name` field, which is rich text field field that includes
the `Job.Name`, `TaskGroup.Name`, and an index value (e.g `exmaple.web[0]`). The index values
are constrained from 0 to `TaskGroup.Count` - 1.

From the configuration and the state, the `TaskGroupReconciler` can build a `reconcileResults` that
the `GenericScheduler` can use to create a `Plan`. The workflow is as follows.

- The `allocRunner` calls `computeGroup` as it currently does.
- Internally, `computeGroup` now calls `NewTaskGroupReconciler` to create a new instance.
- Internally, `TaskGroupReconciler` performs the following tasks during initialization:
    - Creates a `map[stromg]*structs.Allocation` where the map key is the `Allocation.Name`.
    - Creates a slice of `allocSlot` instances with a `len` equal to the `TaskGroup.Count`
    - configuration value.
    - As each `allocSlot` is created, existing `Allocation` instances are added to it's `Candidates`
    - slice based on `Allocation.Name`.
    - `Allocation` instances that don't match an `allocSlot.Name` can immediately be discarded or
      added to the `reconcileResults.stop` slice since they do not target a currently valid slot.
- `computeGroup` resumes and calls `BuildResult`
- Internally, `BuildResult` iterates over each `allocSlot` and calls a domain method for each
  on each instance that is purpose build to return appendable results for each slice field
  the `reconcileResults` requires. This simplifies debugging, because now the set of `Allocation`
  instances being analyzed is limited to what should be a very finite subset.
- `computeGroup` resumes and merges the result of the previous call with the `allocReconciler.results`
  instance.
- Finally, `computeGroup` returns the result of `TaskGroupReconciler.IsDeploymentComplete` to
  the initiating loop found in `computeDeploymentComplete`.

