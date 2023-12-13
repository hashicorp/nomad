/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import faker from 'faker';
import config from 'nomad-ui/config/environment';

const searchIncludesSeed = window.location.search.includes('faker-seed');

if (
  config.environment !== 'test' ||
  config.percy.enabled ||
  searchIncludesSeed
) {
  if (searchIncludesSeed) {
    const params = new URLSearchParams(window.location.search);
    const seed = parseInt(params.get('faker-seed'));
    faker.seed(seed);
  } else {
    faker.seed(1);
  }
} else if (config.environment === 'test') {
  const randomSeed = faker.random.number();
  console.log(
    `No seed specified with faker-seed query parameter, seeding Faker with ${randomSeed}`
  );
  faker.seed(randomSeed);
}

export default faker;
