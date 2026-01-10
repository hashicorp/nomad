/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { computed } from '@ember/object';
import { or } from '@ember/object/computed';

export default class Deployment extends AbstractAbility {
  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'specificNamespaceSupportsFailing'
  )
  canFail;

  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'specificNamespaceSupportsPromoting'
  )
  canPromote;

  @computed('rulesForNamespace.@each.capabilities')
  get specificNamespaceSupportsFailing() {
    return (
      this.namespaceIncludesCapability('submit-job') ||
      this.namespaceIncludesCapability('fail-deployment')
    );
  }

  @computed('rulesForNamespace.@each.capabilities')
  get specificNamespaceSupportsPromoting() {
    return (
      this.namespaceIncludesCapability('submit-job') ||
      this.namespaceIncludesCapability('promote-deployment')
    );
  }
}