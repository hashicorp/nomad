/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class JobsRunTemplatesIndexRoute extends Route {
  @service abilities;
  @service router;
  @service store;

  beforeModel() {
    const hasPermissions = this.abilities.can('write variable', null, {
      namespace: '*',
      path: '*',
    });

    if (!hasPermissions) {
      this.router.transitionTo('jobs');
    }
  }

  model() {
    return this.store.adapterFor('variable').getJobTemplates();
  }

  resetController(controller) {
    controller.set('selectedTemplate', null);
  }
}
