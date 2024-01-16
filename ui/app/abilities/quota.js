/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { alias, and } from '@ember/object/computed';
import { computed } from '@ember/object';

export default class Quota extends AbstractAbility {
  @alias('hasFeatureAndManagement') canConfigureInNamespace;

  @and('quotasIsPresent', 'selfTokenIsManagement')
  hasFeatureAndManagement;

  @computed('features.[]')
  get quotasIsPresent() {
    return this.featureIsPresent('Resource Quotas');
  }
}
