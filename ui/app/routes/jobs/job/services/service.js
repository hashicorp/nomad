/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { addObserver, removeObserver } from '@ember/object/observers';

export default class JobsJobServicesServiceRoute extends Route {
  model({ name = '', level = '' }) {
    const services = this.modelFor('jobs.job')
      .get('services')
      .filter(
        (service) => service.name === name && service.derivedLevel === level,
      );
    return { name, instances: services || [] };
  }

  // Watch the parent job's services collection while this detail route is
  // active. When the watcher updates job.services (e.g. after
  // `nomad service delete`), job.services.length changes and we refresh this
  // route so model() reruns with a fresh filtered snapshot. Without this, the
  // static `instances` array captured at route entry would keep stale
  // references to records that have been unloaded from the store.
  //
  // The observer is added in activate() and torn down in deactivate(), so its
  // lifetime is bounded to the time this route is active. The ember/no-observers
  // rule is disabled because we genuinely need cross-route reactivity here.
  activate() {
    super.activate(...arguments);
    const job = this.modelFor('jobs.job');
    if (!job) return;
    this._servicesObserver = () => {
      if (this.isDestroyed || this.isDestroying) return;
      this.refresh();
    };
    // eslint-disable-next-line ember/no-observers
    addObserver(job, 'services.length', this._servicesObserver);
  }

  deactivate() {
    const job = this.modelFor('jobs.job');
    if (job && this._servicesObserver) {
      removeObserver(job, 'services.length', this._servicesObserver);
    }
    this._servicesObserver = null;
    super.deactivate(...arguments);
  }
}
