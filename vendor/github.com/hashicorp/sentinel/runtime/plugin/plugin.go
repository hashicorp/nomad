// Package plugin contains the interfaces and API for creating Sentinel
// plugins. A Sentinel plugin can provide data dynamically to Sentinel
// policies.
package plugin

import (
	"github.com/hashicorp/sentinel/runtime/gobridge"
)

//go:generate rm -f mock_Import.go
//go:generate mockery -inpkg -note "Generated code. DO NOT MODIFY." -name=Import

// Import plugins allow Go code to satisfy the "import" statement within
// a Sentinel police.
//
// An import behaves just like an external object except that it may also be
// configured. The configuration step allows an import to have custom behavior
// for a policy. For example, this may be used to configure API tokens if the
// import is accessing an API.
type Import interface {
	gobridge.Import

	// Configure is called to configure the plugin before it is accessed.
	// This must be called before any call to Get().
	Configure(map[string]interface{}) error
}
