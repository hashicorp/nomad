/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class DispatchRoute extends Route {
  @service abilities;
  @service router;

  beforeModel() {
    const job = this.modelFor('jobs.job');
    const namespace = job.namespace.get('name');
    if (this.abilities.cannot('dispatch job', null, { namespace })) {
      this.router.transitionTo('jobs.job');
    }
  }

  model() {
    const job = this.modelFor('jobs.job');
    if (!job) return this.router.transitionTo('jobs.job');
    return job;
  }
}
