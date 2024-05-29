/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { pickOne } from '../utils';

export default Factory.extend({
  id: () =>
    `${faker.hacker.verb().replace(/\s/g, '-')}-${faker.random.alphaNumeric(
      5
    )}`,
  name() {
    return this.id;
  },
  description: () =>
    faker.random.number(10) >= 2 ? faker.lorem.sentence() : null,

  policy: `# This policy will always fail. You can temporarily halt all new job updates using this.

  main = rule { false }`,

  scope: 'submit-job',
  enforcementLevel: pickOne(['advisory', 'soft-mandatory', 'hard-mandatory']),
});
