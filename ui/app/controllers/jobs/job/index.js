/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import classic from 'ember-classic-decorator';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { restartableTask, timeout } from 'ember-concurrency';
import Ember from 'ember';

@classic
export default class IndexController extends Controller.extend(
  WithNamespaceResetting
) {
  @service system;
  @service watchList;

  queryParams = [
    {
      currentPage: 'page',
    },
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
    'activeTask',
    'statusMode',
  ];

  currentPage = 1;

  @alias('model') job;

  sortProperty = 'name';
  sortDescending = false;

  @tracked activeTask = null;

  /**
   * @type {('current'|'historical')}
   */
  @tracked
  statusMode = 'current';

  @action
  setActiveTaskQueryParam(task) {
    if (task) {
      this.activeTask = `${task.allocation.id}-${task.name}`;
    } else {
      this.activeTask = null;
    }
  }

  /**
   * @param {('current'|'historical')} mode
   */
  @action
  setStatusMode(mode) {
    this.statusMode = mode;
  }

  @tracked
  childJobsController = new AbortController();

  childJobsQuery(params) {
    this.childJobsController.abort();
    this.childJobsController = new AbortController();

    return this.store
      .query('job', params, {
        adapterOptions: {
          abortController: this.childJobsController,
        },
      })
      .catch((e) => {
        if (e.name !== 'AbortError') {
          console.log('error fetching job ids', e);
        }
        return;
      });
  }

  @tracked childJobs = [];

  resetQueryIndex({ id, namespace }) {
    this.watchList.setIndexFor(`child-jobs-for-${id}-${namespace}`, 1);
  }

  @restartableTask *watchChildJobs(
    { id, namespace },
    throttle = Ember.testing ? 0 : 2000
  ) {
    this.childJobs = [];
    while (true) {
      let params = {
        filter: `ParentID == "${id}"`,
        namespace,
        include_children: true,
      };
      params.index = this.watchList.getIndexFor(
        `child-jobs-for-${id}-${namespace}`
      );

      const childJobs = yield this.childJobsQuery(params);
      if (childJobs) {
        if (childJobs.meta.index) {
          this.watchList.setIndexFor(
            `child-jobs-for-${id}-${namespace}`,
            childJobs.meta.index
          );
        }
        this.childJobs = childJobs;
        yield timeout(throttle);
      }
      if (Ember.testing) {
        break;
      }
    }
  }
}
