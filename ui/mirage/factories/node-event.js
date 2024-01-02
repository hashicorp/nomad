/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide } from '../utils';

const REF_TIME = new Date();
const STATES = provide(10, faker.system.fileExt.bind(faker.system));

export default Factory.extend({
  subsystem: () => faker.helpers.randomize(STATES),
  message: () => faker.lorem.sentence(),
  time: () => faker.date.past(2 / 365, REF_TIME),
  details: null,
});
