/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Model, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  events: hasMany('node-event'),
});
