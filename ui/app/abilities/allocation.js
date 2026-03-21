/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { get } from '@ember/object';

export default class Allocation extends AbstractAbility {
  get canExec() {
    return (
      this.bypassAuthorization ||
      this.selfTokenIsManagement ||
      this.policiesSupportExec
    );
  }

  get policiesSupportExec() {
    return this.rulesForNamespace.some((rules) => {
      let capabilities = get(rules, 'Capabilities') || [];
      return capabilities.includes('alloc-exec');
    });
  }
}
