// +build pro ent

package state

import (
	"testing"

	memdb "github.com/hashicorp/go-memdb"
)

func TestStateStoreSchema_pro(t *testing.T) {
	schema := stateStoreSchema()
	_, err := memdb.NewMemDB(schema)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}
