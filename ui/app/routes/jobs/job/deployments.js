/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import { collect } from '@ember/object/computed';
import { watchRelationship } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import { inject as service } from '@ember/service';

export default class DeploymentsRoute extends Route.extend(WithWatchers) {
  @service store;

  model() {
    const job = this.modelFor('jobs.job');
    return (
      job &&
      RSVP.all([job.get('deployments'), job.get('versions')]).then(() => job)
    );
  }

  startWatchers(controller, model) {
    if (model) {
      controller.set('watchDeployments', this.watchDeployments.perform(model));
      controller.set('watchVersions', this.watchVersions.perform(model));
    }
  }

  @watchRelationship('deployments') watchDeployments;
  @watchRelationship('versions') watchVersions;

  @collect('watchDeployments', 'watchVersions') watchers;
}
