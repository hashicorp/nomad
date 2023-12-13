/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { create, visitable } from 'ember-cli-page-object';

import jobEditor from 'nomad-ui/tests/pages/components/job-editor';

export default create({
  visit: visitable('/jobs/run'),
  editor: jobEditor(),
});
