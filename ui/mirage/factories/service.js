import { Factory, faker } from 'ember-cli-mirage';

export default Factory.extend({
  name: id => `${faker.hacker.noun().dasherize()}-${id}-service`,
});
