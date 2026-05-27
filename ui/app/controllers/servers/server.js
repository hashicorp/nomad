/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';

export default class ServersServerController extends Controller {
  get server() {
    return this.model;
  }
}
