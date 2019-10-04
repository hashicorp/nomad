import faker from 'faker';

const searchIncludesSeed = window.location.search.includes('faker-seed');

// FIXME restore environment check
if (searchIncludesSeed) {
  const params = new URLSearchParams(window.location.search);
  const seed = parseInt(params.get('faker-seed'));
  faker.seed(seed);
} else {
  faker.seed(1);
}

export default faker;
