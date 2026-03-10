/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Model, belongsTo } from 'ember-cli-mirage';

export default Model.extend({
  taskState: belongsTo(),
});
