/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class JobsJobVariablesRoute extends Route {
  @service can;
  @service router;
  @service store;

  beforeModel() {
    if (this.can.cannot('list variables')) {
      this.router.transitionTo(`/jobs/job`);
    }
  }
  async model() {
    const variables = await this.store.findAll('variable');
    return { variables, job: this.modelFor('jobs.job') };
  }
}
