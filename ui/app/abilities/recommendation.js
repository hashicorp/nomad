/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';

export default class Recommendation extends AbstractAbility {
  get canAccept() {
    return this.dynamicApplicationSizingIsPresent && this.hasPermissions;
  }

  get hasPermissions() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.policiesSupportAcceptingOnAnyNamespace
    );
  }

  get policiesSupportAcceptingOnAnyNamespace() {
    return this.capabilitiesForAllNamespaces.includes('submit-job');
  }

  get dynamicApplicationSizingIsPresent() {
    return this.featureIsPresent('Dynamic Application Sizing');
  }
}
