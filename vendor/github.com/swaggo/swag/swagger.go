package swag

import (
	"errors"
	"sync"
)

// Name is a unique name be used to register swag instance.
const Name = "swagger"

var (
	swaggerMu sync.RWMutex
	swag      Swagger
)

// Swagger is a interface to read swagger document.
type Swagger interface {
	ReadDoc() string
}

// Register registers swagger for given name.
func Register(name string, swagger Swagger) {
	swaggerMu.Lock()
	defer swaggerMu.Unlock()
	if swagger == nil {
		panic("swagger is nil")
	}

	if swag != nil {
		panic("Register called twice for swag: " + name)
	}
	swag = swagger
}

// ReadDoc reads swagger document.
func ReadDoc() (string, error) {
	if swag != nil {
		return swag.ReadDoc(), nil
	}
	return "", errors.New("not yet registered swag")
}
