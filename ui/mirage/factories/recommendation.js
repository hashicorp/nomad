/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';

import faker from 'nomad-ui/mirage/faker';

const REF_TIME = new Date();

export default Factory.extend({
  submitTime: () => faker.date.past(2 / 365, REF_TIME) * 1000000,

  afterCreate(recommendation) {
    const base =
      recommendation.resource === 'CPU'
        ? recommendation.task.resources.CPU
        : recommendation.task.resources.MemoryMB;
    const recommendDecrease = faker.random.boolean();
    const directionMultiplier = recommendDecrease ? -1 : 1;

    const value = base + Math.floor(base * 0.5) * directionMultiplier;

    const min = faker.random.number({ min: 5, max: value * 0.4 });
    const max = faker.random.number({ min: value * 0.6, max: value });
    const p99 = faker.random.number({ min: min + (max - min) * 0.8, max });
    const mean = faker.random.number({ min, max: p99 });
    const median = faker.random.number({ min, max: p99 });

    recommendation.update({
      stats: { min, max, p99, mean, median },
      value,
    });
  },
});
