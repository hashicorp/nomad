/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Watchable from './watchable';
import classic from 'ember-classic-decorator';

@classic
export default class PluginAdapter extends Watchable {
  queryParamsToAttrs = {
    type: 'type',
  };
}
