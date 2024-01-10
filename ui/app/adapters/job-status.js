/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Watchable from './watchable';
import { assign } from '@ember/polyfills';

export default class JobStatusAdapter extends Watchable {
  urlForQuery() {
    return `/${this.namespace}/jobs/statuses`;
  }
  query(store, type, query, snapshotRecordArray, options) {
    options = options || {};
    options.adapterOptions = options.adapterOptions || {};
    options.adapterOptions.method = 'POST';
    return super.query(store, type, query, snapshotRecordArray, options);
  }
}
