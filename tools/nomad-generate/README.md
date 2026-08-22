# Boilerplate method generator

Developers are required to implement methods like `Copy`, `Equals`, `Diff`, and
`Merge` for many of the types in the structs package, and this is both
time-consuming and error prone, having resulted in several correctness bugs from
failing to copy objects retrieved from go-memdb or failing to compare objects
during plans.

This is a prototype tool to use `go generate` directives that can generate
methods recursively for deeply nested types automatically, while only having to
annotate the "top level" struct (ex. `structs.Job`). We'll continue to debug and
refine the tool as we go.

## Status

The current tool is still experimental and is expected to change.

## Usage

To mark a package for generation, add a `go:generate` directive one of the
source files of the package as follows:

`//go:generate nomad-generate`

Then add a comment to the docstring for just the top-level type you want to
generate methods for. It must start the line with `nomad-generate:` and finish
with a comma-separated list of method names.

```go
// Foo is a foo that bars.
// nomad-generate: Copy,Diff
type Foo struct {
  Field1 *Bar
  Field2 map[string]Baz
  Field3 []*Qux
}
```

The tool will generate the same methods for all "child" struct and pointer types
found in the top-level type's fields, recursively. So in the example above,
we'll generate the `Copy` and `Diff` methods for the `Foo`, `Bar`, `Baz`, and
`Qux` types.

## How It Works

This tool uses `go/ast` and `go/packages` to read Go code just as the Go
toolchain does, analyzes it to determine a graph of structs that need to
implement the requested methods, and then uses `text/template` to render a
templatized chunk of code for each type and method.

Roughly there are 5 steps.
* Loading all the package files with `go/package`
* Parse the docstrings using `go/doc` to find the `nomad-generate` annotations
  so we know what methods and top-level targets we want.
* Walk the AST and build a "graph" of all types (`ast.TypeSpec`) in the package.
* For each method, trace the graph from each of the top-level type to find all
  its children, and collect these types in a `Result`.
* Send the `Result` object to `text/template` to render it to file.

## Future Work

Future change sets will include:

* Finalizing the methods we can generate, with `Copy` and `Diff` as priority
  targets.
* Replace the existing hand-generated methods.
* Parse struct tags or comments to exclude fields from method generation.
* A `reflect`-based testing tool that can populate a large nested struct for use
  with testing the generated methods.
