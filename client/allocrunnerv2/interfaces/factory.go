package interfaces

import (
	"context"

	"github.com/hashicorp/nomad/client/allocrunnerv2/config"
)

// AllocRunnerFactory is the factory method for retrieving an allocation runner.
type AllocRunnerFactory func(ctx context.Context, config *config.Config) (AllocRunner, error)
