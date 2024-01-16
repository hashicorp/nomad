/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { create, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/access-control'),
  visitTokens: visitable('/access-control/tokens'),
  visitPolicies: visitable('/access-control/policies'),
  visitRoles: visitable('/access-control/roles'),
  visitNamespaces: visitable('/access-control/namespaces'),
});
