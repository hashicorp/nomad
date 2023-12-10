/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory, trait } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';

const REF_TIME = new Date();

export default Factory.extend({
  time: () => faker.date.past(2 / 365, REF_TIME) * 1000000,
  count: () => faker.random.number(10),
  previousCount: () => faker.random.number(10),
  error: () => faker.random.number(10) > 8,
  message: 'Sample message for a job scale event',
  meta: () =>
    faker.random.number(10) < 8
      ? {
          'nomad_autoscaler.count.capped': true,
          'nomad_autoscaler.count.original': 0,
          'nomad_autoscaler.reason_history': ['scaling down because factor is 0.000000'],
        }
      : {},

  afterCreate(scaleEvent) {
    if (scaleEvent.error) {
      scaleEvent.update({ count: null });
    }
  },
});
