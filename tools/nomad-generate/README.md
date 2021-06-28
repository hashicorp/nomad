# Boilerplate method generator for nomad/structs

Developers are required to implement methods like Copy, Equals, Diff,
and Merge for many of the types in the structs package, and this is both
time-consuming and error prone, having resulted in several correctness bugs
from failing to copy objects retrieved from go-memdb or failing to compare
objects during plans.

This is a prototype tool to use with go:generate directives that can generate
methods automatically for our developers. Future change sets will include
documentation for developers and porting the existing  methods to use this tool.
We'll continue to debug and refine the tool as we go.

The current API is not stable and is expected to change. Specific planned changes
are detailed below.

## Usage

To mark a struct in nomad/structs for generation, add a `go:generate` directive
to the source file for the struct. The following example shows how to work with the current
prototype.

`//go:generate -type Job -method=Job.Copy -method Job.Equals -exclude Job.Stop -exclude Job.CreateIndex`

The current prototype expects a single `go:generate` per package. This has been
expedient for prototyping purposes, but the current plan is to refactor to a
separate directive per type approach.

### Flags

- `-type` - The set of types to generate methods for. To target multiple types,
  repeat as separate flags. **Expect this to change to a per directive approach.**
- `-method` - The set of methods to generate for each targeted. Currently, this
  flag requires you to pass methods in the form of `StructName.MethodName` e.g.
  `Job.Equals`. To target multiple methods, repeat as separate flags. To target all
  possible methods, pass `StructName.All`. **Expect this to change to a per directive approach.**
- `-exclude` - The set of fields to exclude when generating code for each targeted. Currently,
  this  flag requires you to pass fields in the form of `StructName.FieldName` e.g.
  `Job.CreateIndex`. To target multiple fields, repeat as separate flags.
  **Expect this to change to a per directive approach.**
- `-packageDir` - The relative path to the source directory containing structs to
  generate methods for. This currently defaults to the well known location of the
  `nomad/structs` package, but could be overridden to test against other packages.
  
