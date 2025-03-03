/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model from '@ember-data/model';
import { attr, belongsTo, hasMany } from '@ember-data/model';
export default class DynamicHostVolumeModel extends Model {
  @attr('string') name;
  @attr('string') path;
  @belongsTo('namespace') namespace;
  @belongsTo('node') node;
  @attr('string') pluginID;
  @attr() constraints;
  @hasMany('allocation', { async: false }) allocations;
}
