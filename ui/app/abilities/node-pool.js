/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';

export default class NodePool extends AbstractAbility {
  get canConfigureInNamespace() {
    return this.hasFeatureAndManagement;
  }

  get hasFeatureAndManagement() {
    return this.nodePoolGovernanceIsPresent && this.selfTokenIsManagement;
  }

  get nodePoolGovernanceIsPresent() {
    return this.featureIsPresent('Node Pools Governance');
  }
}
