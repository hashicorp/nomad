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
    queryParams.namespace = '*';

    /* eslint-disable ember/no-controller-access-in-routes */
    let filter = this.controllerFor('jobs.index').filter;
    if (filter) {
      queryParams.filter = filter;
    }
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
    const knownKeys = [
      {
        key: 'Name',
        example: 'Name == my-job',
      },
      {
        key: 'Status',
        example: 'Status != running',
      },
      {
        key: 'StatusDescription',
        example: 'StatusDescription contains "progress deadline"',
      },
      {
        key: 'Region',
        example: 'Region != global',
      },
      {
        key: 'NodePool',
        example: 'NodePool is not empty',
      },
      {
        key: 'Namespace',
        example: 'Namespace !== myNamespace',
      },
      {
        key: 'Version',
        example: 'Version != 0',
      },
      {
        key: 'Priority',
        example: 'Priority != 50',
      },
      {
        key: 'Stop',
        example: 'Stop == false',
      },
      {
        key: 'Type',
        example: 'Type contains sys',
      },
      {
        key: 'ID',
        example: 'ID == myJob',
      },
      {
        key: 'AllAtOnce',
        example: 'AllAtOnce == true',
      },
      {
        key: 'Datacenters',
        example: 'dc1 in Datacenters',
      },
      {
        key: 'Dispatched',
        example: 'Dispatched == false',
      },
      {
        key: 'ConsulToken',
        example: 'ConsulToken is not empty',
      },
      {
        key: 'ConsulNamespace',
        example: 'ConsulNamespace == myNamespace',
      },
      {
        key: 'VaultToken',
        example: 'VaultToken is not empty',
      },
      {
        key: 'VaultNamespace',
        example: 'VaultNamespace == myNamespace',
      },
      {
        key: 'NomadTokenID',
        example: 'NomadTokenID != myToken',
      },
      {
        key: 'Stable',
        example: 'Stable == false',
      },
      {
        key: 'SubmitTime',
        example: 'SubmitTime == 1716387219559280000',
      },
      {
        key: 'CreateIndex',
        example: 'CreateIndex != 10',
      },
      {
        key: 'ModifyIndex',
        example: 'ModifyIndex == 30',
      },
      {
        key: 'JobModifyIndex',
        example: 'JobModifyIndex == 40',
      },
    ];
    error.errors?.forEach((err) => {
      this.notifications.add({
        title: err.title,
        message: err.detail,
        color: 'critical',
        timeout: 8000,
      });
    });

    let err = error.errors?.objectAt(0);
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

      let humanized = err.detail || '';

      // Two ways we can help users here:
      // 1. They slightly mis-typed a key, so we should offer a correction
      // 2. They tried a key that didn't look like anything we know of, so we can suggest keys they might try
      let correction = null;
      let suggestion = null;

      const keyMatch = err.detail.match(
        /couldn't find key: struct field with name "([^"]+)"/
      );
      if (keyMatch && keyMatch[1]) {
        const incorrectKey = keyMatch[1];
        const correctKey = knownKeys.find(
          (key) =>
            key.key ===
            `${incorrectKey.charAt(0).toUpperCase()}${incorrectKey
              .slice(1)
              .toLowerCase()}`
        )?.key;
        if (correctKey) {
          correction = {
            incorrectKey,
            correctKey,
          };
        } else {
          humanized = `Did you mistype a key? Valid keys include:`;
          suggestion = knownKeys;
        }
      }

      return {
        error: {
          humanized,
          correction,
          suggestion,
        },
      };
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
