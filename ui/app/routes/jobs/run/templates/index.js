/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class JobsRunTemplatesIndexRoute extends Route {
  @service can;
  @service router;
  @service store;

  beforeModel() {
    const hasPermissions = this.can.can('write variable', null, {
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
