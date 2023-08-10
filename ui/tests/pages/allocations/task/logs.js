/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { create, isPresent, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/allocations/:id/:name/logs'),
  visitParentJob: visitable('/jobs/:id/allocations'),

  hasTaskLog: isPresent('[data-test-task-log]'),
  sidebarIsPresent: isPresent('.sidebar.task-context-sidebar'),
});
