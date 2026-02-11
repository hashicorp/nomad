package admission

import (
	"context"

	"github.com/hashicorp/nomad/nomad/structs"
)

type AdmissionController interface {
	Start(context.Context)
	AdmitJob(*structs.Job) ([]error, error)
}
