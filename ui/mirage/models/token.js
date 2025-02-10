/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Model, belongsTo, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  policies: hasMany(),
  roles: hasMany(),
});
