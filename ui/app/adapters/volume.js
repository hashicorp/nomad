/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import WatchableNamespaceIDs from './watchable-namespace-ids';
import classic from 'ember-classic-decorator';

@classic
export default class VolumeAdapter extends WatchableNamespaceIDs {
  queryParamsToAttrs = {
    type: 'type',
    plugin_id: 'plugin.id',
  };
}
