/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class VariablesNewRoute extends Route {
  @service store;
  async model(params) {
    const namespaces = await this.store.peekAll('namespace');
    return this.store.createRecord('variable', {
      path: params.path,
      namespace: namespaces[0]?.id,
    });
  }
  resetController(controller, isExiting) {
    // If the user navigates away from /new, clear the path
    controller.set('path', null);
    if (isExiting) {
      // If user didn't save, delete the freshly created model
      if (controller?.model?.isNew) {
        try {
          controller.model.unloadRecord();
        } catch {
          // Record may already be disconnected during teardown.
        }
      }
    }
  }
}
