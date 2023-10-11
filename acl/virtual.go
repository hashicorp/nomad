// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

var ServerACL = initServerACL()

func initServerACL() *ACL {
	aclObj, err := NewACL(false, []*Policy{})
	if err != nil {
		panic(err)
	}
	aclObj.agent = PolicyRead
	aclObj.server = PolicyWrite
	return aclObj
}
