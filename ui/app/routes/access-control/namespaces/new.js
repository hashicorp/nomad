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
    let defaultMeta = {};

    let defaultNodePoolConfig = null;
    if (this.can.can('configure-in-namespace node-pool')) {
      defaultNodePoolConfig = this.store.createFragment(
        'ns-node-pool-configuration',
        {
          Default: 'default',
          Allowed: [],
          Disallowed: null,
        }
      );
    }

    let defaultCapabilities = this.store.createFragment('ns-capabilities', {
      DisabledTaskDrivers: ['raw_exec'],
    });

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
        controller.model.capabilities?.unloadRecord();
        controller.model.nodePoolConfiguration?.unloadRecord();
        controller.model.unloadRecord();
      }
    }
  }
}
