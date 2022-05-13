package nomad

import (
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

var mvs MockVariableStore

type MockVariableStore struct {
	m          sync.RWMutex
	backingMap map[string]*structs.SecureVariable
	s          *Server
	l          hclog.Logger
}

func (mvs *MockVariableStore) List(prefix string) []*structs.SecureVariableStub {
	mvs.l.Info("***** List *****")
	mvs.m.Lock()
	mvs.m.Unlock()
	if len(mvs.backingMap) == 0 {
		return nil
	}
	vars := make([]*structs.SecureVariableStub, 0, len(mvs.backingMap))
	for p, sVar := range mvs.backingMap {
		if strings.HasPrefix(p, prefix) {
			outVar := sVar.Stub()
			vars = append(vars, &outVar)
		}
	}
	return vars
}
func (mvs *MockVariableStore) Add(p string, bag structs.SecureVariable) {
	mvs.l.Info("***** Add *****")
	mvs.m.Lock()
	mvs.m.Unlock()
	nv := bag.Copy()
	mvs.backingMap[p] = nv
}

func (mvs *MockVariableStore) Get(p string) *structs.SecureVariable {
	mvs.l.Info("***** Get *****")
	var out *structs.SecureVariable
	mvs.m.Lock()
	defer mvs.m.Unlock()

	if v, ok := mvs.backingMap[p]; ok {
		out = v.Copy()
	} else {
		return nil
	}
	return out
}

// Delete removes a key from the store. Removing a non-existent key is a no-op
func (mvs *MockVariableStore) Delete(p string) {
	mvs.l.Info("***** Delete *****")
	mvs.m.Lock()
	defer mvs.m.Unlock()
	delete(mvs.backingMap, p)
}

// Delete removes a key from the store. Removing a non-existent key is a no-op
func (mvs *MockVariableStore) Reset() {
	mvs.l.Info("***** Reset *****")
	mvs.m.Lock()
	mvs.m.Unlock()
	mvs.backingMap = make(map[string]*structs.SecureVariable)
}

func NewMockVariableStore(s *Server, l hclog.Logger) {
	l.Info("***** Initializing mock variables backend *****")
	mvs.m.Lock()
	mvs.m.Unlock()
	mvs.backingMap = make(map[string]*structs.SecureVariable)
	mvs.s = s
	mvs.l = l
}

func SV_List(args *structs.SecureVariablesListRequest, out *structs.SecureVariablesListResponse) {
	out.Data = mvs.List(args.Prefix)
	out.QueryMeta.KnownLeader = true
	// TODO: Would be nice to at least have a forward moving number for index
	// even in testing.
	out.QueryMeta.Index = 999
	out.QueryMeta.LastContact = 19
}

func SV_Upsert(args *structs.SecureVariablesUpsertRequest, out *structs.SecureVariablesUpsertResponse) {
	nv := args.Data.Copy()
	mvs.Add(nv.Path, *nv)
	// TODO: Would be nice to at least have a forward moving number for index
	// even in testing.
	out.WriteMeta.Index = 9999
}
func SV_Read(args *structs.SecureVariablesReadRequest, out *structs.SecureVariablesReadResponse) {
	out.Data = mvs.Get(args.Path)
	// TODO: Would be nice to at least have a forward moving number for index
	// even in testing.
	out.Index = 9999
	out.QueryMeta.KnownLeader = true
	out.QueryMeta.Index = 999
	out.QueryMeta.LastContact = 19
}
func SV_Delete(args *structs.SecureVariablesDeleteRequest, out *structs.SecureVariablesDeleteResponse) {
	mvs.Delete(args.Path)
	out.WriteMeta.Index = 9999
}
