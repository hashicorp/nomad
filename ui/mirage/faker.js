import faker from 'faker';
import config from 'nomad-ui/config/environment';

const searchIncludesSeed = window.location.search.includes('faker-seed');

if (config.environment !== 'test' || searchIncludesSeed) {
  if (searchIncludesSeed) {
    const params = new URLSearchParams(window.location.search);
    const seed = parseInt(params.get('faker-seed'));
    faker.seed(seed);
  } else {
    faker.seed(1);
  }
}

export default faker;
