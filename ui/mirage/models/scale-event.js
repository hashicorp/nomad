/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Model, belongsTo } from 'ember-cli-mirage';

export default Model.extend({
  taskGroupScale: belongsTo(),
});
