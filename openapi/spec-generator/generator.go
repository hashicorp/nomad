package main

// Generator iterates over the input source and configuration, and aggregates a
// data model a that can be used to render an openapi from the template.
type Generator struct {
	Spec Spec
}
