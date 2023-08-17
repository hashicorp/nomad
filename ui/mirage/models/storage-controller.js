/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { Model, belongsTo } from 'ember-cli-mirage';

export default Model.extend({
  job: belongsTo(),
  node: belongsTo(),
});
