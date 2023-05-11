/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { Model, belongsTo, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  taskGroup: belongsTo(),
  recommendations: hasMany(),
  services: hasMany('service-fragment'),
});
