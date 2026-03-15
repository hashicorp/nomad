/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Model, hasMany } from 'miragejs';

export default Model.extend({
  taskStates: hasMany('task-state'),
  taskResources: hasMany('task-resource'),
});
