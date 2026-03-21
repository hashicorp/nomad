/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { copy } from 'ember-copy';
import Service from '@ember/service';

let list = {};

export default class WatchListService extends Service {
  get _list() {
    return copy(list, true);
  }

  get list() {
    return this._list;
  }

  constructor() {
    super(...arguments);
    list = {};
  }

  getIndexFor(url) {
    return list[url] || 1;
  }

  setIndexFor(url, value) {
    list[url] = +value;
  }

  jobsIndexIDsController = new AbortController();
  jobsIndexDetailsController = new AbortController();
}
