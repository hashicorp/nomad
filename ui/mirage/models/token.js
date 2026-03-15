/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Model, belongsTo, hasMany } from 'miragejs';

export default Model.extend({
  policies: hasMany(),
  roles: hasMany(),
});
