/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import WatchableNamespaceIDs from './watchable-namespace-ids';

export default class JobSummaryAdapter extends WatchableNamespaceIDs {
  urlForFindRecord(id, type, hash) {
    return super.urlForFindRecord(id, 'job', hash, 'summary');
  }
}
