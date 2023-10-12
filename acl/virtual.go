// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

var ClientACL = initClientACL()
var ServerACL = initServerACL()

func initClientACL() *ACL {
	aclObj, err := NewACL(false, []*Policy{})
	if err != nil {
		panic(err)
	}
	aclObj.client = PolicyWrite
	aclObj.agent = PolicyRead
	aclObj.server = PolicyRead
	return aclObj
}

func initServerACL() *ACL {
	aclObj, err := NewACL(false, []*Policy{})
	if err != nil {
		panic(err)
	}
	aclObj.agent = PolicyRead
	aclObj.server = PolicyWrite
	return aclObj
}
