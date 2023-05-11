/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { Model, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  events: hasMany('node-event'),
});
