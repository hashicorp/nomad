/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';
import { collect } from '@ember/object/computed';
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
    const namespace = transition.to.queryParams.namespace;
    const name = params.job_name;
    const fullId = JSON.stringify([name, namespace || 'default']);

    const jobPromise = this.store
      .findRecord('job', fullId)
      .then((job) => {
        return job.get('allocations').then(() => job);
      })
      .catch(notifyError(this));

    const xtermImport = import('xterm').then((module) => module.Terminal);

    return Promise.all([jobPromise, xtermImport]);
  }

  setupController(controller, [job, Terminal]) {
    super.setupController(controller, job);
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

  @collect('watch', 'watchAllocations') watchers;
}
