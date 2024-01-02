/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide } from '../utils';
import { dasherize } from '@ember/string';
import { pickOne } from '../utils';

const ON_UPDATE = ['default', 'ignore', 'ignore_warnings'];

export default Factory.extend({
  name: (id) => `${dasherize(faker.hacker.noun())}-${id}-service`,
  portLabel: () => dasherize(faker.hacker.noun()),
  onUpdate: faker.helpers.randomize(ON_UPDATE),
  provider: () => pickOne(['nomad', 'consul']),
  tags: () => {
    if (!faker.random.boolean()) {
      return provide(
        faker.random.number({ min: 0, max: 2 }),
        faker.hacker.noun.bind(faker.hacker.noun)
      );
    } else {
      return null;
    }
  },
  Connect: {
    SidecarService: {
      Proxy: {
        Upstreams: [
          {
            DestinationName: dasherize(faker.hacker.noun()),
            LocalBindPort: faker.random.number({ min: 5000, max: 60000 }),
          },
        ],
      },
    },
  },
});
