/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';

export default class SentinelPolicy extends AbstractAbility {
  get canRead() {
    return this.hasFeatureAndManagement;
  }

  get canList() {
    return this.hasFeatureAndManagement;
  }

  get canWrite() {
    return this.hasFeatureAndManagement;
  }

  get canUpdate() {
    return this.hasFeatureAndManagement;
  }

  get canDestroy() {
    return this.hasFeatureAndManagement;
  }

  get hasAuthority() {
    return this.bypassAuthorization || this.selfTokenIsManagement;
  }

  get hasFeatureAndManagement() {
    return this.sentinelIsPresent && this.hasAuthority;
  }

  get sentinelIsPresent() {
    return this.featureIsPresent('Sentinel Policies');
  }
}
