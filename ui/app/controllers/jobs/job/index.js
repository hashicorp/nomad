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
    console.log('cjQ', params);

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

  @restartableTask *watchChildJobs(
    { id, namespace },
    throttle = Ember.testing ? 0 : 5000
  ) {
    while (true) {
      console.log('wt', id, namespace);
      let params = {
        filter: `ParentID == "${id}"`, // TODO: "and Namespace == ..."
        include_children: true,
      };
      params.index = this.watchList.getIndexFor(`child-jobs-for-${id}`); // TODO: maybe I should stick with the URL convention here
      console.log('paramind', params.index);
      const childJobs = yield this.childJobsQuery(params);
      console.log('childYobs', childJobs);
      if (childJobs) {
        if (childJobs.meta.index) {
          this.watchList.setIndexFor(
            `child-jobs-for-${id}`,
            childJobs.meta.index
          );
        }
        this.childJobs = childJobs;
        yield timeout(throttle);
      }
      //  else {
      //   // This returns undefined on page change / cursorAt change, resulting from the aborting of the old query.
      //   yield timeout(throttle);
      //   this.watchJobs.perform(this.jobIDs, throttle);
      //   continue;
      // }
      if (Ember.testing) {
        break;
      }
    }
  }
}
