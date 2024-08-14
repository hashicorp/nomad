/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class SettingsUserSettingsRoute extends Route {
  @service system;
  // Make sure to load namespaces
  async model() {
    let defaults = await this.system.defaults;
    return {
      namespaces: this.store.findAll('namespace'),
      nodePools: this.store.findAll('node-pool'),
      defaults,
    };
  }
}
