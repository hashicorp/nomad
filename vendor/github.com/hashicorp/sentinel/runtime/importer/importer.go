// Package importer contains interfaces and structures that are used for
// implementing the "import" functionality. The specification states that
// the implementation of how imports are packaged and loaded is runtime-specific.
//
// This package and subpackages contain a number of types of importers.
package importer

import (
	"github.com/hashicorp/sentinel-sdk"
)

// Importer is responsible for processing "import" statements.
type Importer interface {
	// Import is called to import the package with the given name.
	// This must return a non-nil value if error is nil. If the named package is not
	// found or doesn't exist, an error must be returned. A nil value
	// returned here will cause an immediate runtime error for the
	// executing policy.
	Import(string) (sdk.Import, error)
}
