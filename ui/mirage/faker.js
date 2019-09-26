import faker from 'faker';
import Ember from 'ember';

if (!Ember.testing) {
  if (window.location.search.includes('faker-seed')) {
    const params = new URLSearchParams(window.location.search);
    const seed = parseInt(params.get('faker-seed'));
    faker.seed(seed);
  } else {
    faker.seed(1);
  }
}

export default faker;
