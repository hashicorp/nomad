/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import WatchableNamespaceIDs from './watchable-namespace-ids';

export default class DynamicHostVolumeAdapter extends WatchableNamespaceIDs {
  pathForType = () => 'volumes';

  urlForFindRecord(fullID) {
    const [id, namespace] = JSON.parse(fullID);

    let url = `/${this.namespace}/volume/host/${id}`;

    if (namespace && namespace !== 'default') {
      url += `?namespace=${namespace}`;
    }

    return url;
  }
}
