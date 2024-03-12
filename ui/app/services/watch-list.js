/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// import { computed } from '@ember/object';
// import { readOnly } from '@ember/object/computed';
// import { copy } from 'ember-copy';
import Service from '@ember/service';
import { tracked } from '@glimmer/tracking';

// let list = {};

export default class WatchListService extends Service {
  // @computed
  // get _list() {
  //   return copy(list, true);
  // }

  jobsIndexIDsController = new AbortController();
  jobsIndexDetailsController = new AbortController();

  // @readOnly('_list') list;
  @tracked list = {};

  // constructor() {
  //   super(...arguments);
  //   list = {};
  // }

  getIndexFor(url) {
    return this.list[url] || 1;
  }

  setIndexFor(url, value) {
    this.list[url] = +value;
    this.list = { ...this.list };
  }

  /**
   * When we paginate or otherwise manually change queryParams for our jobs index,
   * we want our requests to return immediately. This means we need to clear out
   * any previous indexes that are associated with the jobs index.
   */
  clearJobsIndexIndexes() {
    // If it starts with /v1/jobs/statuses, remove it
    let keys = Object.keys(this.list);
    keys.forEach((key) => {
      if (key.startsWith('/v1/jobs/statuses')) {
        delete this.list[key];
      }
    });
  }
}
