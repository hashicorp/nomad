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
  @attr() requestedCapabilities;
  @attr('number') capacityBytes;

  get idWithNamespace() {
    return `${this.plainId}@${this.namespace}`;
  }

  get capabilities() {
    let capabilities = [];
    if (this.requestedCapabilities) {
      this.requestedCapabilities.forEach((capability) => {
        capabilities.push({
          access_mode: capability.AccessMode,
          attachment_mode: capability.AttachmentMode,
        });
      });
    }
    return capabilities;
  }
}
