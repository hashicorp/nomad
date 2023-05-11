/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { Factory } from 'ember-cli-mirage';

import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  id: i => (i === 0 ? 'default' : `namespace-${i}`),

  name() {
    return this.id;
  },

  hash: () => faker.random.uuid(),
  description: '',
});
