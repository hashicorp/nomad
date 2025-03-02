/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import WatchableNamespaceIDs from './watchable-namespace-ids';
import classic from 'ember-classic-decorator';

@classic
export default class DynamicHostVolumeAdapter extends WatchableNamespaceIDs {
  pathForType = () => 'volume/host';

  urlForFindRecord(fullID) {
    const [id, namespace] = JSON.parse(fullID);

    let url = `/v1/${this.pathForType()}/${id}`;

    if (namespace && namespace !== 'default') {
      url += `?namespace=${namespace}`;
    }

    return url;
  }
  // queryParamsToAttrs = {
  //   type: 'type',
  //   plugin_id: 'plugin.id',
  // };
}
