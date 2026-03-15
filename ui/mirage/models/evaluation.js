/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Model, hasMany, belongsTo } from 'miragejs';

export default Model.extend({
  relatedEvals: hasMany('evaluation-stub'),
});
