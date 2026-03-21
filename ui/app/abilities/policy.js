/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';

export default class Policy extends AbstractAbility {
  get canRead() {
    return this.selfTokenIsManagement;
  }

  get canList() {
    return this.selfTokenIsManagement;
  }

  get canWrite() {
    return this.selfTokenIsManagement;
  }

  get canUpdate() {
    return this.selfTokenIsManagement;
  }

  get canDestroy() {
    return this.selfTokenIsManagement;
  }
}
