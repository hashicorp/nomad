/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class AccessControlTokensNewRoute extends Route {
  @service can;
  @service router;

  beforeModel() {
    if (this.can.cannot('write token')) {
      this.router.transitionTo('/access-control/tokens');
    }
  }

  async model() {
    let token = await this.store.createRecord('token', {
      name: '',
      type: 'client',
    });
    return {
      token,
      policies: await this.store.findAll('policy'),
      roles: await this.store.findAll('role'),
    };
  }

  resetController(controller, isExiting) {
    if (isExiting) {
      // If user didn't save, delete the freshly created model
      if (controller.model.token.isNew) {
        controller.model.token.destroyRecord();
      }
    }
  }
}
