package ids

import (
	"github.com/hashicorp/go-uuid"
)

type UUID = string

func NewUUID() UUID {
	id, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}
	return id
}

func ShortUUID(id UUID) string {
	return NewUUID()[0:4]
}
