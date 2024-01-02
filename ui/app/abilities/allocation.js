/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';

export default class Allocation extends AbstractAbility {
  @or('bypassAuthorization', 'selfTokenIsManagement', 'policiesSupportExec')
  canExec;

  @computed('rulesForNamespace.@each.capabilities')
  get policiesSupportExec() {
    return this.rulesForNamespace.some((rules) => {
      let capabilities = get(rules, 'Capabilities') || [];
      return capabilities.includes('alloc-exec');
    });
  }
}
