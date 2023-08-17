/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { Model, hasMany, belongsTo } from 'ember-cli-mirage';

export default Model.extend({
  relatedEvals: hasMany('evaluation-stub'),
});
