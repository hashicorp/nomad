/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class AccessControlNamespacesNewRoute extends Route {
  @service can;
  @service router;

  beforeModel() {
    if (this.can.cannot('write namespace')) {
      this.router.transitionTo('/access-control/namespaces');
    }
  }

  async model() {
    let defaultCapabilities = {
      DisabledTaskDrivers: ['raw_exec'],
    };
    let defaultMeta = {};
    let defaultNodePoolConfig = null;

    if (this.can.can('configure-node-pools namespace')) {
      defaultNodePoolConfig = {
        Default: 'default',
        Allowed: [],
      };
    }

    return await this.store.createRecord('namespace', {
      name: '',
      description: '',
      capabilities: defaultCapabilities,
      meta: defaultMeta,
      quota: '',
      nodePoolConfiguration: defaultNodePoolConfig,
    });
  }

  resetController(controller, isExiting) {
    if (isExiting) {
      // If user didn't save, delete the freshly created model
      if (controller.model.isNew) {
        controller.model.destroyRecord();
      }
    }
  }
}
