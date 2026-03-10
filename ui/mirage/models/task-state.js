/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Model, belongsTo, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  allocation: belongsTo(),
  events: hasMany('task-event'),
});
