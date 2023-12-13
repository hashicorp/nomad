/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import WatchableNamespaceIDs from './watchable-namespace-ids';
import classic from 'ember-classic-decorator';

@classic
export default class JobSummaryAdapter extends WatchableNamespaceIDs {
  urlForFindRecord(id, type, hash) {
    return super.urlForFindRecord(id, 'job', hash, 'summary');
  }
}
