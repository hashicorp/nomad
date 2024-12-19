/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';

export default Factory.extend({
  cron: '* * * * *',
  task: null,
});
