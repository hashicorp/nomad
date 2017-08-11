package contexts

// Context is a type which is searchable via a unique identifier.
type Context string

const (
	Allocs Context = "allocs"
	Evals  Context = "evals"
	Jobs   Context = "jobs"
	Nodes  Context = "nodes"
	All    Context = ""
)
