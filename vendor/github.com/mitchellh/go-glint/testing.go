package glint

import (
	"github.com/mitchellh/go-testing-interface"
)

// TestRender renders the component using the string renderer and returns
// the string. This is a test helper function for writing components.
func TestRender(t testing.T, c Component) string {
	// Note that nothing here fails at the moment so the t param above is
	// unneeded but we're gonna keep it around in case we need it in the future
	// so we don't have to break API.
	r := &StringRenderer{}
	d := New()
	d.SetRenderer(r)
	d.Append(c)
	d.RenderFrame()
	return r.Builder.String()
}
