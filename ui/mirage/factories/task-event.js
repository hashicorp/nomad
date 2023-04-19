/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide } from '../utils';

const REF_TIME = new Date();
const STATES = provide(10, faker.system.fileExt.bind(faker.system));

export default Factory.extend({
  type: () => faker.helpers.randomize(STATES),

  signal: () => '',
  exitCode: () => null,
  time: () => faker.date.past(2 / 365, REF_TIME) * 1000000,

  message: () => faker.lorem.sentence(),
});
