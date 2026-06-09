# Loop Detection Dependency Graph

This folder provides a small dependency graph implementation for detecting circular dependencies between jobs.

Package name: `depgraph`

## Public API

```go
type Graph interface {
    AddNodes(nodeID string, dependencies ...string) error
    RemoveNode(nodeID string) error
}
```

Constructor:

```go
g := depgraph.New()
```

## Internal Model

The implementation keeps both of these structures:

1. An array of linked lists (`allLists`) for all nodes.
2. A map from node ID to its linked list (`byNode`).

It also tracks adjacency for efficient checks:

- `deps`: node -> direct dependencies
- `dependents`: node -> direct dependents

## Behavior

### AddNodes

`AddNodes(nodeID, deps...)` does the following:

1. Validates IDs are non-empty.
2. Rejects self-dependency (`nodeID` depending on itself).
3. Creates missing nodes on demand.
4. Ignores duplicate dependencies in the same call.
5. Detects cycles before adding each edge.

Cycle rule:

- When adding `A -> B`, it checks whether `B` already reaches `A`.
- If yes, the new edge would create a loop and returns an error.

### RemoveNode

`RemoveNode(nodeID)` does the following:

1. Returns `ErrNodeNotFound` if the node does not exist.
2. Returns `ErrNodeIsDependency` if any other node depends on it.
3. Removes the node if it has no dependents.
4. Prunes orphan dependency branches recursively.

Orphan pruning means if a removed node had dependencies that are no longer required by anyone else, those dependency nodes are also removed.

## Errors

- `ErrEmptyNodeID`
- `ErrSelfDependency`
- `ErrNodeNotFound`
- `ErrNodeIsDependency`

## Example

```go
g := depgraph.New()

_ = g.AddNodes("jobA", "jobB", "jobC")
_ = g.AddNodes("jobB", "jobD")

// Would fail: jobD -> jobA closes a cycle jobA -> jobB -> jobD -> jobA
if err := g.AddNodes("jobD", "jobA"); err != nil {
    // handle cycle error
}

// Would fail while jobA depends on jobB
if err := g.RemoveNode("jobB"); err != nil {
    // handle ErrNodeIsDependency
}
```

## Tests

See test cases in `loop_detection_test.go`.
Each test includes an ASCII graph diagram before the test function.
