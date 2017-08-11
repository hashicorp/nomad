package contexts

type Context string

const (
	Alloc Context = "allocs"
	Eval  Context = "evals"
	Job   Context = "jobs"
	Node  Context = "nodes"
	All   Context = ""
)
