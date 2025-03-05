/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import WatchableNamespaceIDs from './watchable-namespace-ids';
import classic from 'ember-classic-decorator';

@classic
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
