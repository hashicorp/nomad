/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { get } from '@ember/object';

export default class Job extends AbstractAbility {
  get canRun() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.policiesSupportRunning
    );
  }

  get canScale() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.specificNamespaceSupportsRunning ||
      this.policiesSupportScaling
    );
  }

  get canRead() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.specificNamespaceSupportsReading ||
      this.policiesSupportReading
    );
  }

  // TODO: A person can also see all jobs if their token grants read access to all namespaces,
  // but given the complexity of namespaces and policy precedence, there isn't a good quick way
  // to confirm this.
  get canListAll() {
    return this.bypassAuthorization || this.selfTokenIsManagement;
  }

  get canDispatch() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.policiesSupportDispatching
    );
  }

  get canStop() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.specificNamespaceSupportsStopping
    );
  }

  get canPurge() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.specificNamespaceSupportsPurging
    );
  }

  get canRevert() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.specificNamespaceSupportsReverting
    );
  }

  get canStart() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.specificNamespaceSupportsRunning
    );
  }

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

  get policiesSupportRunning() {
    return this.policyNamespacesIncludePermissions(
      this.token.selfTokenPolicies,
      ['submit-job', 'register-job'],
    );
  }

  get policiesSupportReading() {
    return this.policyNamespacesIncludePermissions(
      this.token.selfTokenPolicies,
      ['read-job'],
    );
  }

  get specificNamespaceSupportsRunning() {
    return (
      this.namespaceIncludesCapability('submit-job') ||
      this.namespaceIncludesCapability('register-job')
    );
  }

  get specificNamespaceSupportsReading() {
    return this.namespaceIncludesCapability('read-job');
  }

  get specificNamespaceSupportsStopping() {
    return (
      this.namespaceIncludesCapability('submit-job') ||
      this.namespaceIncludesCapability('deregister-job') ||
      this.namespaceIncludesCapability('purge-job')
    );
  }

  get specificNamespaceSupportsPurging() {
    return (
      this.namespaceIncludesCapability('submit-job') ||
      this.namespaceIncludesCapability('purge-job')
    );
  }

  get policiesSupportScaling() {
    return this.namespaceIncludesCapability('scale-job');
  }

  get policiesSupportDispatching() {
    return this.namespaceIncludesCapability('dispatch-job');
  }

  get specificNamespaceSupportsReverting() {
    return (
      this.namespaceIncludesCapability('submit-job') ||
      this.namespaceIncludesCapability('revert-job')
    );
  }
}
