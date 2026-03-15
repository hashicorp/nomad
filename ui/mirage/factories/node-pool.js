/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'miragejs';

export default Factory.extend({
  name: (i) => `node-pool-${i}`,
  description: (i) => `describe node-pool-${i}`,
  meta: {},
  schedulerConfiguration: {},
});
