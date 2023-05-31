/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { Factory } from 'ember-cli-mirage';

export default Factory.extend({
  name: (i) => `node-pool-${i}`,
  description: (i) => `describe node-pool-${i}`,
  meta: {},
  schedulerConfiguration: {},
});
