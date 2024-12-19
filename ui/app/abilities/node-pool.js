/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { alias, and } from '@ember/object/computed';
import { computed } from '@ember/object';

export default class NodePool extends AbstractAbility {
  @alias('hasFeatureAndManagement') canConfigureInNamespace;

  @and('nodePoolGovernanceIsPresent', 'selfTokenIsManagement')
  hasFeatureAndManagement;

  @computed('features.[]')
  get nodePoolGovernanceIsPresent() {
    return this.featureIsPresent('Node Pools Governance');
  }
}
