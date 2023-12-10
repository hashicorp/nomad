/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class DispatchRoute extends Route {
  @service can;

  beforeModel() {
    const job = this.modelFor('jobs.job');
    const namespace = job.namespace.get('name');
    if (this.can.cannot('dispatch job', null, { namespace })) {
      this.transitionTo('jobs.job');
    }
  }

  model() {
    const job = this.modelFor('jobs.job');
    if (!job) return this.transitionTo('jobs.job');
    return job;
  }
}
