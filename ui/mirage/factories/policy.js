/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  id: () => faker.hacker.verb(),
  name() {
    return this.id;
  },
  description: () => (faker.random.number(10) >= 2 ? faker.lorem.sentence() : null),
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
