/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Watchable from './watchable';
import classic from 'ember-classic-decorator';

@classic
export default class PluginAdapter extends Watchable {
  queryParamsToAttrs = {
    type: 'type',
  };
  // Over in serializers/plugin.js, we prepend csi/ as part of the hash ID for request resolution reasons.
  // However, this is not part of the actual ID stored in the database and we should treat it like a regular, unescaped
  // path segment.
  urlForFindRecord() {
    let url = super.urlForFindRecord(...arguments);
    return url.replace('csi%2F', 'csi/');
  }
}
