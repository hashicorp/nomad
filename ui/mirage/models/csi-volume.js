/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { Model, belongsTo, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  plugin: belongsTo('csi-plugin'),
  writeAllocs: hasMany('allocation'),
  readAllocs: hasMany('allocation'),
  allocations: hasMany('allocation'),
});
