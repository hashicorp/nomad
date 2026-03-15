/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Model, belongsTo, hasMany } from 'miragejs';

export default Model.extend({
  taskGroup: belongsTo(),
  recommendations: hasMany(),
  services: hasMany('service-fragment'),
  schedule: belongsTo('task-schedule'),
});
