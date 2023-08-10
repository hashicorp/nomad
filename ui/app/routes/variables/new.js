/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';

export default class VariablesNewRoute extends Route {
  async model(params) {
    const namespaces = await this.store.peekAll('namespace');
    return this.store.createRecord('variable', {
      path: params.path,
      namespace: namespaces.objectAt(0)?.id,
    });
  }
  resetController(controller, isExiting) {
    // If the user navigates away from /new, clear the path
    controller.set('path', null);
    if (isExiting) {
      // If user didn't save, delete the freshly created model
      if (controller.model.isNew) {
        controller.model.destroyRecord();
      }
    }
  }
}
