/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';

import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  id: (i) => (i === 0 ? 'default' : `namespace-${i}`),

  name() {
    return this.id;
  },

  hash: () => faker.random.uuid(),
  description: '',
});
