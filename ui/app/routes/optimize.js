/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import classic from 'ember-classic-decorator';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { next } from '@ember/runloop';
import RSVP from 'rsvp';

@classic
export default class OptimizeRoute extends Route {
  @service can;
  @service store;

  beforeModel() {
    if (this.can.cannot('accept recommendation')) {
      this.transitionTo('jobs');
    }
  }

  async model() {
    const summaries = await this.store.findAll('recommendation-summary');
    const jobs = await RSVP.all(summaries.mapBy('job'));
    const [namespaces] = await RSVP.all([
      this.store.findAll('namespace'),
      ...jobs
        .filter((job) => job)
        .filterBy('isPartial')
        .map((j) => j.reload()),
    ]);

    return {
      summaries: summaries.sortBy('submitTime').reverse(),
      namespaces,
    };
  }

  @action
  reachedEnd() {
    this.store.unloadAll('recommendation-summary');

    next(() => {
      this.transitionTo('optimize');
      this.refresh();
    });
  }
}
