/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';

export default class Quota extends AbstractAbility {
  get canConfigureInNamespace() {
    return this.hasFeatureAndManagement;
  }

  get hasFeatureAndManagement() {
    return this.quotasIsPresent && this.selfTokenIsManagement;
  }

  get quotasIsPresent() {
    return this.featureIsPresent('Resource Quotas');
  }
}
