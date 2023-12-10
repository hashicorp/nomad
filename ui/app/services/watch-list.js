/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';
import { readOnly } from '@ember/object/computed';
import { copy } from 'ember-copy';
import Service from '@ember/service';

let list = {};

export default class WatchListService extends Service {
  @computed
  get _list() {
    return copy(list, true);
  }

  @readOnly('_list') list;

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
}
