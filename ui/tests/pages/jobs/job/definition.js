/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { create, isPresent, visitable, clickable } from 'ember-cli-page-object';

import jobEditor from 'nomad-ui/tests/pages/components/job-editor';
import error from 'nomad-ui/tests/pages/components/error';

export default create({
  visit: visitable('/jobs/:id/definition'),

  jsonViewer: isPresent('[data-test-definition-view]'),
  editor: jobEditor(),

  edit: clickable('[data-test-edit-job]'),

  error: error(),
});
