/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Model, belongsTo, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  plugin: belongsTo('csi-plugin'),
  writeAllocs: hasMany('allocation'),
  readAllocs: hasMany('allocation'),
  allocations: hasMany('allocation'),
});
