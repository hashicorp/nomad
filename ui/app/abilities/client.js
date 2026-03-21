/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { get } from '@ember/object';

export default class Client extends AbstractAbility {
  // Map abilities to policy options (which are coarse for nodes)
  // instead of specific behaviors.
  get canRead() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.policiesIncludeNodeRead
    );
  }

  get canWrite() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.policiesIncludeNodeWrite
    );
  }

  get policiesIncludeNodeRead() {
    return policiesIncludePermissions(this.token.selfTokenPolicies, [
      'read',
      'write',
    ]);
  }

  get policiesIncludeNodeWrite() {
    return policiesIncludePermissions(this.token.selfTokenPolicies, ['write']);
  }
}

function policiesIncludePermissions(policies = [], permissions = []) {
  const policyList =
    typeof policies?.toArray === 'function' ? policies.toArray() : policies;

  // For each policy record, extract the Node policy
  const nodePolicies = policyList
    .map((policy) => get(policy, 'rulesJSON.Node.Policy'))
    .filter(Boolean);

  // Check for requested permissions
  return nodePolicies.some((policy) => permissions.includes(policy));
}
