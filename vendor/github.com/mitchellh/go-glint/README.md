# go-glint [![Godoc](https://godoc.org/github.com/mitchellh/go-glint?status.svg)](https://godoc.org/github.com/mitchellh/go-glint)

Glint is a component-based UI framework specifically targeted towards
command-line interfaces. This allows you to create highly dynamic CLI interfaces
using shared, easily testable components. Glint uses a Flexbox implementation
to make it easy to lay out components in the CLI, including paddings, margins,
and more.

**API Status: Unstable.** We're still actively working on the API and
may change it in backwards incompatible ways. See the roadmap section in
particular for work that may impact the API. In particular, we have 
integrated this library into [Waypoint](https://github.com/hashicorp/waypoint),
and the experience of using this library in the real world will likely drive major
changes.

## Example

The example below shows a simple dynamic counter:

```go
func main() {
	var counter uint32
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			atomic.AddUint32(&counter, 1)
		}
	}()

	d := glint.New()
	d.Append(
		glint.Style(
			glint.TextFunc(func(rows, cols uint) string {
				return fmt.Sprintf("%d tests passed", atomic.LoadUint32(&counter))
			}),
			glint.Color("green"),
		),
	)
	d.Render(context.Background())
}
```

Output:

![Example](https://user-images.githubusercontent.com/1299/92431533-9baf8000-f14c-11ea-94ad-8ff97ed26fec.gif)

## Roadmap

Glint is still an early stage project and there is a lot that we want to
improve on. This may introduce some backwards incompatibilities but we are
trying to stabilize the API as quickly as possible.

* **Non-interactive interfaces.** We want to add support for rendering to
non-interactive interfaces and allowing components to provide custom behavior
in these cases. For now, users of Glint should detect non-interactivity and
avoid using Glint.

* **Windows PowerShell and Cmd.** Glint works fine in ANSI-compatible terminals
on Windows, but doesn't work with PowerShell and Cmd. We want to make this
work.

* **Dirty tracking.** Glint currently rerenders the entire frame on each
tick. I'd like components to be able to report if there are changes (if they
are "dirty") and need to be rerendered. We could then more efficiently
recalculate layouts and rerender outputs.

* **User Input.** Glint should be able to query for user input and render
this within its existing set of components.

* **Expose styling to custom renderers.** Currently the `Style` component
is a special-case for the terminal renderer to render colors. I'd like to expose
the styles in a way that other renderers could use it in some meaningful way.

## Thanks

This library is heavily inspired by the [Ink project](https://github.com/vadimdemedes/ink).
I saw this project and thought that having a central render loop along with
a full layout engine was a fantastic idea. Most of my projects are in Go
so I wanted to be able to realize these benefits with Go. Thank you!
