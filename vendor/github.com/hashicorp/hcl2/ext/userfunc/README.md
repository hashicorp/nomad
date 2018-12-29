# HCL User Functions Extension

This HCL extension allows a calling application to support user-defined
functions.

Functions are defined via a specific block type, like this:

```hcl
function "add" {
  params = [a, b]
  result = a + b
}

function "list" {
  params         = []
  variadic_param = items
  result         = items
}
```

The extension is implemented as a pre-processor for `cty.Body` objects. Given
a body that may contain functions, the `DecodeUserFunctions` function searches
for blocks that define functions and returns a functions map suitable for
inclusion in a `hcl.EvalContext`. It also returns a new `cty.Body` that
contains the remainder of the content from the given body, allowing for
further processing of remaining content.

For more information, see [the godoc reference](http://godoc.org/github.com/hashicorp/hcl2/ext/userfunc).
