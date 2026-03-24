/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { action } from '@ember/object';
import Controller from '@ember/controller';

export default class ClientMonitorController extends Controller {
  queryParams = ['level'];
  level = 'info';

  @action
  setLevel(level) {
    this.level = level;
  }
}
