/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'miragejs';
import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  name() {
    return this.id;
  },
  args: () => [],
  command: () => faker.hacker.phrase(),
});
