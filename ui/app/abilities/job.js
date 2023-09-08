/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';

export default class Job extends AbstractAbility {
  @or('bypassAuthorization', 'selfTokenIsManagement', 'policiesSupportRunning')
  canRun;

  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'specificNamespaceSupportsRunning',
    'policiesSupportScaling'
  )
  canScale;

  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'specificNamespaceSupportsReading',
    'policiesSupportReading'
  )
  canRead;

  // TODO: A person can also see all jobs if their token grants read access to all namespaces,
  // but given the complexity of namespaces and policy precedence, there isn't a good quick way
  // to confirm this.
  @or('bypassAuthorization', 'selfTokenIsManagement')
  canListAll;

  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'policiesSupportDispatching'
  )
  canDispatch;

  policyNamespacesIncludePermissions(policies = [], permissions = []) {
    // For each policy record, extract all policies of all namespaces
    const allNamespacePolicies = policies
      .toArray()
      .filter((policy) => get(policy, 'rulesJSON.Namespaces'))
      .map((policy) => get(policy, 'rulesJSON.Namespaces'))
      .flat()
      .map((namespace = {}) => {
        return namespace.Capabilities;
      })
      .flat()
      .compact();

    // Check for requested permissions
    return allNamespacePolicies.some((policy) => {
      return permissions.includes(policy);
    });
  }

  @computed('token.selfTokenPolicies.[]')
  get policiesSupportRunning() {
    return this.policyNamespacesIncludePermissions(
      this.token.selfTokenPolicies,
      ['submit-job']
    );
  }

  @computed('token.selfTokenPolicies.[]')
  get policiesSupportReading() {
    return this.policyNamespacesIncludePermissions(
      this.token.selfTokenPolicies,
      ['read-job']
    );
  }

  @computed('rulesForNamespace.@each.capabilities')
  get specificNamespaceSupportsRunning() {
    return this.namespaceIncludesCapability('submit-job');
  }

  @computed('rulesForNamespace.@each.capabilities')
  get specificNamespaceSupportsReading() {
    return this.namespaceIncludesCapability('read-job');
  }

  @computed('rulesForNamespace.@each.capabilities')
  get policiesSupportScaling() {
    return this.namespaceIncludesCapability('scale-job');
  }

  @computed('rulesForNamespace.@each.capabilities')
  get policiesSupportDispatching() {
    return this.namespaceIncludesCapability('dispatch-job');
  }
}
