/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import {
  watchRecord,
  watchRelationship,
} from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import { inject as service } from '@ember/service';

export default class VersionsRoute extends Route.extend(WithWatchers) {
  @service store;

  queryParams = {
    diffVersion: {
      refreshModel: true,
    },
  };

  // model() {
  //   console.log('model refire');
  //   const job = this.modelFor('jobs.job');
  //   const versions = job.get('versions');
  //   return job && job.get('versions').then(() => job);
  // }

  async model(params) {
    console.log('model refire', params);
    const job = this.modelFor('jobs.job');

    // Force reload of the versions relationship
    await job.hasMany('versions').reload({
      adapterOptions: {
        diffVersion: params.diffVersion,
      },
    });

    console.log('versions on the job', job.versions);

    return job;
  }

  // async model(params) {
  //   console.log('model refire', params);
  //   const job = this.modelFor('jobs.job');

  //   // Fetch versions based on diffVersion
  //   const versions = await this.store.query('job-version', {
  //     job: job.id,
  //     diffs: true,
  //     diffVersion: params.diffVersion
  //   });

  //   console.log('versions as fetched', versions);
  //   console.log('versions on the job', job.versions);

  //   // // Set the versions on the job model
  //   // job.set('versions', versions);

  //   return job;
  // }

  startWatchers(controller, model) {
    if (model) {
      controller.set('watcher', this.watch.perform(model));
      controller.set('watchVersions', this.watchVersions.perform(model));
    }
  }

  @watchRecord('job') watch;
  @watchRelationship('versions') watchVersions;

  @collect('watch', 'watchVersions') watchers;
}
