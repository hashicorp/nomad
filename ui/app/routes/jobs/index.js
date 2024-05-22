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
import Ember from 'ember';

const DEFAULT_THROTTLE = 2000;

export default class IndexRoute extends Route.extend(
  WithWatchers,
  WithForbiddenState
) {
  @service store;
  @service watchList;
  @service notifications;

  queryParams = {
    qpNamespace: {
      refreshModel: true,
    },
    cursorAt: {
      refreshModel: true,
    },
    pageSize: {
      refreshModel: true,
    },
    filter: {
      refreshModel: true,
    },
  };

  hasBeenInitialized = false;

  getCurrentParams() {
    let queryParams = this.paramsFor(this.routeName); // Get current query params
    if (queryParams.cursorAt) {
      queryParams.next_token = queryParams.cursorAt;
    }
    queryParams.per_page = queryParams.pageSize;

    /* eslint-disable ember/no-controller-access-in-routes */
    let filter = this.controllerFor('jobs.index').filter;
    if (filter) {
      queryParams.filter = filter;
    }
    // namespace
    queryParams.namespace = queryParams.qpNamespace;
    delete queryParams.qpNamespace;
    delete queryParams.pageSize;
    delete queryParams.cursorAt;

    return { ...queryParams };
  }

  async model(/*params*/) {
    let currentParams = this.getCurrentParams();
    this.watchList.jobsIndexIDsController.abort();
    this.watchList.jobsIndexIDsController = new AbortController();
    try {
      let jobs = await this.store.query('job', currentParams, {
        adapterOptions: {
          abortController: this.watchList.jobsIndexIDsController,
        },
      });
      return RSVP.hash({
        jobs,
        namespaces: this.store.findAll('namespace'),
        nodePools: this.store.findAll('node-pool'),
      });
    } catch (error) {
      try {
        notifyForbidden(this)(error);
      } catch (secondaryError) {
        return this.handleErrors(error);
      }
    }
    return {};
  }

  /**
   * @typedef {Object} HTTPError
   * @property {string} stack
   * @property {string} message
   * @property {string} name
   * @property {HTTPErrorDetail[]} errors
   */

  /**
   * @typedef {Object} HTTPErrorDetail
   * @property {string} status - HTTP status code
   * @property {string} title
   * @property {string} detail
   */

  /**
   * Handles HTTP errors by returning an appropriate message based on the HTTP status code and details in the error object.
   *
   * @param {HTTPError} error
   * @returns {Object}
   */
  handleErrors(error) {
    const knownKeys = {
      Name: 'Name',
      Status: 'Status',
      StatusDescription: 'StatusDescription',
      Region: 'Region',
      NodePool: 'NodePool',
      Namespace: 'Namespace',
      Version: 'Version',
      Priority: 'Priority',
      Stop: 'Stop',
      Type: 'Type',
      ID: 'ID',
      AllAtOnce: 'AllAtOnce',
      Datacenters: 'Datacenters',
      Dispatched: 'Dispatched',
      ConsulToken: 'ConsulToken',
      ConsulNamespace: 'ConsulNamespace',
      VaultToken: 'VaultToken',
      VaultNamespace: 'VaultNamespace',
      NomadTokenID: 'NomadTokenID',
      Stable: 'Stable',
      SubmitTime: 'SubmitTime',
      CreateIndex: 'CreateIndex',
      ModifyIndex: 'ModifyIndex',
      JobModifyIndex: 'JobModifyIndex',
    };

    error.errors?.forEach((err) => {
      this.notifications.add({
        title: err.title,
        message: err.detail,
        color: 'critical',
        timeout: 8000,
      });
    });

    let err = error.errors[0];
    // if it's an innocuous-enough seeming "You mistyped something while searching" error,
    // handle it with a notification and don't throw. Otherwise, throw.
    if (
      err?.detail.includes("couldn't find key") ||
      err?.detail.includes('failed to read filter expression')
    ) {
      this.watchList.jobsIndexDetailsController.abort();
      this.watchList.jobsIndexIDsController.abort();
      // eslint-disable-next-line
      this.controllerFor('jobs.index').set('jobIDs', []);
      // eslint-disable-next-line
      this.controllerFor('jobs.index').set('jobs', []);
      // eslint-disable-next-line
      this.controllerFor('jobs.index').watchJobs.cancelAll();
      // eslint-disable-next-line
      this.controllerFor('jobs.index').watchJobIDs.cancelAll();

      let humanizedError = err.detail || '';
      let errorLink = null;

      if (humanizedError.includes('failed to read filter expression')) {
        errorLink = {
          label: 'Learn more about Filter Expressions',
          url: 'https://developer.hashicorp.com/nomad/api-docs#creating-expressions',
        };
      } else {
        const keyMatch = err.detail.match(
          /couldn't find key: struct field with name "([^"]+)"/
        );
        if (keyMatch && keyMatch[1]) {
          const incorrectKey = keyMatch[1];
          const correctKey =
            knownKeys[
              incorrectKey.charAt(0).toUpperCase() +
                incorrectKey.slice(1).toLowerCase()
            ];
          if (correctKey) {
            humanizedError = `Did you mean "${correctKey}"?`;
          } else {
            let possibleKeys = Object.values(knownKeys).join('", "');
            humanizedError = `Did you mistype a key? Valid keys include "${possibleKeys}".`;
          }
        }
      }

      return { error: humanizedError, errorLink };
    } else {
      throw error;
    }
  }

  setupController(controller, model) {
    super.setupController(controller, model);

    if (!this.hasBeenInitialized) {
      controller.parseFilter();
    }
    this.hasBeenInitialized = true;

    if (!model.jobs) {
      return;
    }

    controller.set('nextToken', model.jobs.meta.nextToken);
    controller.set('jobQueryIndex', model.jobs.meta.index);
    controller.set('jobAllocsQueryIndex', model.jobs.meta.allocsIndex); // Assuming allocsIndex is your meta key for job allocations.
    controller.set(
      'jobIDs',
      model.jobs.map((job) => {
        return {
          id: job.plainId,
          namespace: job.belongsTo('namespace').id(),
        };
      })
    );

    // Now that we've set the jobIDs, immediately start watching them
    controller.watchJobs.perform(
      controller.jobIDs,
      Ember.testing ? 0 : DEFAULT_THROTTLE,
      'update'
    );
    // And also watch for any changes to the jobIDs list
    controller.watchJobIDs.perform(
      this.getCurrentParams(),
      Ember.testing ? 0 : DEFAULT_THROTTLE
    );
  }

  startWatchers(controller) {
    controller.set('namespacesWatch', this.watchNamespaces.perform());
  }

  @action
  willTransition(transition) {
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
