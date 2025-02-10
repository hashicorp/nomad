/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  // Extra randomness appended to not conflict with the otherwise-uniq'd policies generated
  // in factories.token.afterCreate
  id: () =>
    `${faker.hacker.verb().replace(/\s/g, '-')}-${faker.random.alphaNumeric(
      5
    )}`,
  name() {
    return this.id;
  },
  description: () =>
    faker.random.number(10) >= 2 ? faker.lorem.sentence() : null,
  rules: `# Allow read only access to the default namespace
namespace "default" {
    policy = "read"
}

node {
    policy = "read"
}`,
  rulesJSON: () => ({
    Node: {
      Policy: 'read',
    },
  }),
});
