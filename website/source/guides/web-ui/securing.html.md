---
layout: "guides"
page_title: "Web UI"
sidebar_current: "guides-web-ui-securing"
description: |-
  Learn how to use ACLs to secure the Web UI
---

# Securing the Web UI with ACLs

By default, all features—read and write—are available to all users of the Web UI. By using [Access Control Lists](/guides/security/acl.html), it is possible to lock down what users get access to which features.

## Browsing the Web UI Without an Access Control Token

When a user browses the Web UI without specifying an access control token, they assume the rules of the [anonymous policy](/guides/security/acl.html#set-an-anonymous-policy-optional-). Since Nomad ACLs use a default-deny model, if ACLs are enabled and no anonymous policy is authored, the Web UI will show unauthorized messages on every page other than the settings page.

[![Not authorized to see jobs][img-jobs-list-unauthorized]][img-jobs-list-unauthorized]

## Setting an Access Control Token

From the ACL Tokens page, which is accessible from the top-right menu, you can set your access control token via the token Secret ID.

This token is saved to local storage and can be manually cleared from the ACL Tokens page.

[![ACL token page][img-acl-token]][img-acl-token]

[![ACL token set][img-acl-token-set]][img-acl-token-set]


[img-jobs-list-unauthorized]: /assets/images/guide-ui-jobs-list-unauthorized.png
[img-acl-token]: /assets/images/guide-ui-acl-token.png
[img-acl-token-set]: /assets/images/guide-ui-acl-token-set.png
