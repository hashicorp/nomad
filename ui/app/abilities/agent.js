/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { get } from '@ember/object';

export default class Client extends AbstractAbility {
  get canRead() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.policiesIncludeAgentReadOrWrite
    );
  }

  get policiesIncludeAgentReadOrWrite() {
    const policies = (get(this, 'token.selfTokenPolicies') || [])
      .toArray()
      .map((policy) => get(policy, 'rulesJSON.Agent.Policy'))
      .compact();

    return policies.some((policy) => policy === 'read' || policy === 'write');
  }
}
