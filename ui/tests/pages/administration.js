/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { create, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/administration'),
  visitTokens: visitable('/administration/tokens'),
  visitPolicies: visitable('/administration/policies'),
  visitRoles: visitable('/administration/roles'),
  visitNamespaces: visitable('/administration/namespaces'),
  visitSentinelPolicies: visitable('/administration/sentinel-policies'),
});
