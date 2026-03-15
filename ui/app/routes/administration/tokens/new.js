/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class AccessControlTokensNewRoute extends Route {
  @service store;
  @service abilities;
  @service router;

  beforeModel() {
    if (this.abilities.cannot('write token')) {
      this.router.transitionTo('/administration/tokens');
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
      const token = controller?.model?.token;
      if (token?.isNew) {
        try {
          token.unloadRecord();
        } catch {
          // During teardown/transition races a token may already be disconnected.
          // In that case there is nothing left to clean up.
        }
      }
    }
  }
}
