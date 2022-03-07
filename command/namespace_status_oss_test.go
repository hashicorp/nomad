//go:build !ent
// +build !ent

package command

import "github.com/hashicorp/nomad/api"

func testQuotaSpec() *api.QuotaSpec {
	panic("not implemented - enterprise only")
}
