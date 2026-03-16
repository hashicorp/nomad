/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import { Terminal } from 'xterm';
import notifyError from 'nomad-ui/utils/notify-error';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import {
  watchRecord,
  watchRelationship,
} from 'nomad-ui/utils/properties/watch';
import classic from 'ember-classic-decorator';

@classic
export default class ExecRoute extends Route.extend(WithWatchers) {
  @service store;
  @service token;

  serialize(model) {
    return { job_name: model.get('plainId') };
  }

  model(params, transition) {
    const namespace = transition?.to?.queryParams?.namespace || 'default';
    const name = params.job_name;
    const fullId = JSON.stringify([name, namespace || 'default']);

    const findJobInStore = (jobs) => {
      return jobs.find((job) => {
        const compositeId = job.get('id');

        if (!compositeId) {
          return false;
        }

        try {
          const [plainId, ns] = JSON.parse(compositeId);
          return plainId === name && (ns || 'default') === namespace;
        } catch {
          return false;
        }
      });
    };

    const jobPromise = this.store
      .findRecord('job', fullId)
      .catch(() => this.store.findAll('job').then(findJobInStore))
      .then((job) => {
        if (!job) {
          const error = new Error(
            `Job ${name} not found in namespace ${namespace}`
          );
          error.code = '404';
          throw error;
        }

        // Ensure we hydrate the full job payload (including task groups)
        // before building Exec UI state from allocation/task relationships.
        return job
          .reload()
          .catch(() => job)
          .then(() => job.get('allocations'))
          .then((allocations) => {
            // Fallback for relationship-linking mismatches: load allocations
            // directly so Exec can still derive task groups.
            if (!allocations?.length) {
              return this.store.findAll('allocation', { reload: true });
            }
            return allocations;
          })
          .then(() => job);
      })
      .catch(notifyError(this));

    return Promise.all([jobPromise, Terminal]);
  }

  setupController(controller, [job, Terminal]) {
    super.setupController(controller, job);
    controller.set('fallbackAllocations', this.store.peekAll('allocation'));
    controller.setUpTerminal(Terminal);
  }

  startWatchers(controller, model) {
    if (model) {
      controller.set('watcher', this.watch.perform(model));
      controller.set('watchAllocations', this.watchAllocations.perform(model));
    }
  }

  @watchRecord('job') watch;
  @watchRelationship('allocations') watchAllocations;

  get watchers() {
    return [this.watch, this.watchAllocations];
  }
}
