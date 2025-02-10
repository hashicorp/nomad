/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import WatchableNamespaceIDs from './watchable-namespace-ids';
import classic from 'ember-classic-decorator';

@classic
export default class VolumeAdapter extends WatchableNamespaceIDs {
  // Over in serializers/volume.js, we prepend csi/ as part of the hash ID for request resolution reasons.
  // However, this is not part of the actual ID stored in the database and we should treat it like a regular, unescaped
  // path segment.
  urlForFindRecord() {
    let url = super.urlForFindRecord(...arguments);
    return url.replace('csi%2F', 'csi/');
  }

  queryParamsToAttrs = {
    type: 'type',
    plugin_id: 'plugin.id',
  };
}
