/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Model, belongsTo, hasMany } from 'miragejs';

export default Model.extend({
  jobScale: belongsTo(),
  events: hasMany('scale-event'),
});
