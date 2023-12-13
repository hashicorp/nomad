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
}
