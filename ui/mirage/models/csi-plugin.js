/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Model, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  nodes: hasMany('storage-node'),
  controllers: hasMany('storage-controller'),
});
