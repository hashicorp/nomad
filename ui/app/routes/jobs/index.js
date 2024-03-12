/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import { collect } from '@ember/object/computed';
import { watchAll } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import { action } from '@ember/object';

export default class IndexRoute extends Route.extend(
  WithWatchers,
  WithForbiddenState
) {
  @service store;
  @service watchList;

  // perPage = 10;

  queryParams = {
    qpNamespace: {
      refreshModel: true,
    },
    cursorAt: {
      refreshModel: true,
    },
  };

  hasBeenInitialized = false;

  getCurrentParams() {
    let queryParams = this.paramsFor(this.routeName); // Get current query params
    queryParams.next_token = queryParams.cursorAt;
    delete queryParams.cursorAt; // TODO: hacky, should be done in the serializer/adapter?
    return { ...queryParams };
  }

  async model(/*params*/) {
    let currentParams = this.getCurrentParams(); // TODO: how do these differ from passed params?
    this.watchList.jobsIndexIDsController.abort();
    this.watchList.jobsIndexIDsController = new AbortController();
    let jobs = await this.store
      .query('job', currentParams, {
        adapterOptions: {
          method: 'GET', // TODO: default
          queryType: 'initialize',
          abortController: this.watchList.jobsIndexIDsController,
          modifyURL: false,
        },
      })
      .catch(notifyForbidden(this));
    return RSVP.hash({
      jobs,
      namespaces: this.store.findAll('namespace'),
      nodePools: this.store.findAll('node-pool'),
    });
  }

  setupController(controller, model) {
    super.setupController(controller, model);
    // TODO: consider re-instating this. This is setting them and then their order gets shuffled.
    // controller.set('jobs', model.jobs);
    controller.set('nextToken', model.jobs.meta.nextToken);
    controller.set(
      'jobIDs',
      model.jobs.map((job) => {
        return {
          id: job.plainId,
          namespace: job.belongsTo('namespace').id(),
        };
      })
    );

    // Note: we should remove the indexes from the watch-list for jobs index queries if we've already initialized, since
    // if we explicitly change our queryParams we want to start from scratch, unindexed
    this.watchList.clearJobsIndexIndexes();

    // TODO: maybe do these in controller constructor?
    // Now that we've set the jobIDs, immediately start watching them
    // eslint-disable-next-line
    this.controller.watchJobs.perform(controller.jobIDs, 2000, 'update');
    // And also watch for any changes to the jobIDs list
    // eslint-disable-next-line
    this.controller.watchJobIDs.perform(this.getCurrentParams(), 2000);

    this.hasBeenInitialized = true;
  }

  startWatchers(controller) {
    controller.set('namespacesWatch', this.watchNamespaces.perform());
  }

  @action
  willTransition(transition) {
    // TODO: Something is preventing jobs -> job -> jobs -> job.
    if (!transition.intent.name?.startsWith(this.routeName)) {
      this.watchList.jobsIndexDetailsController.abort();
      this.watchList.jobsIndexIDsController.abort();
      // eslint-disable-next-line
      this.controller.watchJobs.cancelAll();
      // eslint-disable-next-line
      this.controller.watchJobIDs.cancelAll();
    }
    this.cancelAllWatchers();
    return true;
  }

  // Determines if we should be put into a loading state (jobs/loading.hbs)
  // This is a useful page for when you're first initializing your jobs list,
  // but overkill when we paginate / change our queryParams. We should handle that
  // with in-compnent loading/skeleton states instead.
  @action
  loading() {
    return !this.hasBeenInitialized; // allows the loading template to be shown
  }

  @watchAll('namespace') watchNamespaces;
  @collect('watchNamespaces') watchers;
}
