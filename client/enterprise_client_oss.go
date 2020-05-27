// +build !ent

package client

// EnterpriseClient holds information and methods for enterprise functionality
type EnterpriseClient struct{}

func newEnterpriseClient() *EnterpriseClient {
	return &EnterpriseClient{}
}

// SetFeatures is used for enterprise builds to configure enterprise features
func (ec *EnterpriseClient) SetFeatures(features uint64) {
	return
}
