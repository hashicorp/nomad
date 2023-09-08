/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';

export default class Client extends AbstractAbility {
  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'policiesIncludeAgentReadOrWrite'
  )
  canRead;

  @computed('token.selfTokenPolicies.[]')
  get policiesIncludeAgentReadOrWrite() {
    const policies = (get(this, 'token.selfTokenPolicies') || [])
      .toArray()
      .map((policy) => get(policy, 'rulesJSON.Agent.Policy'))
      .compact();

    return policies.some((policy) => policy === 'read' || policy === 'write');
  }
}
