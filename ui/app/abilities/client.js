/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';
import classic from 'ember-classic-decorator';

@classic
export default class Client extends AbstractAbility {
  // Map abilities to policy options (which are coarse for nodes)
  // instead of specific behaviors.
  @or('bypassAuthorization', 'selfTokenIsManagement', 'policiesIncludeNodeRead')
  canRead;

  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'policiesIncludeNodeWrite'
  )
  canWrite;

  @computed('token.selfTokenPolicies.[]')
  get policiesIncludeNodeRead() {
    return policiesIncludePermissions(this.get('token.selfTokenPolicies'), [
      'read',
      'write',
    ]);
  }

  @computed('token.selfTokenPolicies.[]')
  get policiesIncludeNodeWrite() {
    return policiesIncludePermissions(this.get('token.selfTokenPolicies'), [
      'write',
    ]);
  }
}

function policiesIncludePermissions(policies = [], permissions = []) {
  // For each policy record, extract the Node policy
  const nodePolicies = policies
    .toArray()
    .map((policy) => get(policy, 'rulesJSON.Node.Policy'))
    .compact();

  // Check for requested permissions
  return nodePolicies.some((policy) => permissions.includes(policy));
}
