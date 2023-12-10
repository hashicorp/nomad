/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { computed } from '@ember/object';
import { and, or } from '@ember/object/computed';

export default class Recommendation extends AbstractAbility {
  @and('dynamicApplicationSizingIsPresent', 'hasPermissions')
  canAccept;

  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'policiesSupportAcceptingOnAnyNamespace'
  )
  hasPermissions;

  @computed('capabilitiesForAllNamespaces.[]')
  get policiesSupportAcceptingOnAnyNamespace() {
    return this.capabilitiesForAllNamespaces.includes('submit-job');
  }

  @computed('features.[]')
  get dynamicApplicationSizingIsPresent() {
    return this.featureIsPresent('Dynamic Application Sizing');
  }
}
