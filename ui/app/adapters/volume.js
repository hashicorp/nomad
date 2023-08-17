/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
