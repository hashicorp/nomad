/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { alias, and, or } from '@ember/object/computed';
import { computed } from '@ember/object';

export default class SentinelPolicy extends AbstractAbility {
  @alias('hasFeatureAndManagement') canRead;
  @alias('hasFeatureAndManagement') canList;
  @alias('hasFeatureAndManagement') canWrite;
  @alias('hasFeatureAndManagement') canUpdate;
  @alias('hasFeatureAndManagement') canDestroy;

  @or('bypassAuthorization', 'selfTokenIsManagement')
  hasAuthority;

  @and('sentinelIsPresent', 'hasAuthority')
  hasFeatureAndManagement;

  @computed('features.[]')
  get sentinelIsPresent() {
    return this.featureIsPresent('Sentinel Policies');
  }
}
