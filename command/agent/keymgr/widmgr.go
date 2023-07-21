package keymgr

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

type IdentityToken struct {
	Name string
	JWT  string
	Exp  time.Time
}

type WIDMgrConfig struct {
	NodeSecret  string
	Region      string
	RPC         RPCer
	PubKeyCache *PubKeyCache
	Logger      hclog.Logger
}

// WIDMgr fetches and validates workload identities.
type WIDMgr struct {
	nodeSecret  string
	region      string
	rpc         RPCer
	pubKeyCache *PubKeyCache
	logger      hclog.Logger
}

func NewWIDMgr(c WIDMgrConfig) *WIDMgr {
	return &WIDMgr{
		nodeSecret:  c.NodeSecret,
		region:      c.Region,
		rpc:         c.RPC,
		pubKeyCache: c.PubKeyCache,
		logger:      c.Logger.Named("idmgr"),
	}
}

func (m *WIDMgr) GetIdentities(ctx context.Context, minIndex uint64, req []structs.WorkloadIdentityRequest) ([]IdentityToken, error) {
	args := structs.AllocIdentitiesRequest{
		Identities: req,
		QueryOptions: structs.QueryOptions{
			Region:        m.region,
			MinQueryIndex: minIndex,
			AllowStale:    true,
			AuthToken:     m.nodeSecret,
		},
	}
	reply := structs.AllocIdentitiesResponse{}
	if err := m.rpc.RPC("Alloc.GetIdentities", &args, &reply); err != nil {
		return nil, err
	}

	//TODO what to do about rejections?
	if len(reply.Rejections) > 0 {
		return nil, fmt.Errorf("some signing requests were rejected: %d", len(reply.Rejections))
	}

	ids := make([]IdentityToken, 0, len(reply.SignedIdentities))

	for _, sid := range reply.SignedIdentities {
		claims, err := m.pubKeyCache.ParseJWT(ctx, sid.JWT)
		if err != nil {
			//TODO what to do about errors? add to rejections?
			m.logger.Error("error parsing or validating jwt", "error", err, "jwt", sid.JWT, "sid", sid.IdentityName)
			continue
		}

		ids = append(ids, IdentityToken{
			Name: sid.IdentityName,
			JWT:  sid.JWT,
			Exp:  claims.Expiry.Time(),
		})
	}

	return ids, nil
}
