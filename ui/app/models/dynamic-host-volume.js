/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model from '@ember-data/model';
import { attr, belongsTo, hasMany } from '@ember-data/model';
export default class DynamicHostVolumeModel extends Model {
  @attr('string') plainId;
  @attr('string') name;
  @attr('string') path;
  @attr('string') namespace;
  @attr('string') state;
  @belongsTo('node') node;
  @attr('string') pluginID;
  @attr() constraints;
  @attr('date') createTime;
  @attr('date') modifyTime;
  @hasMany('allocation', { async: false }) allocations;

  get idWithNamespace() {
    return `${this.plainId}@${this.namespace}`;
  }
}
