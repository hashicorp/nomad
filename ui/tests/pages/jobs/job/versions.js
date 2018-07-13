import { create, collection, text, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/jobs/:id/versions'),

  versions: collection('[data-test-version]', {
    text: text(),
    stability: text('[data-test-version-stability]'),
    submitTime: text('[data-test-version-submit-time]'),
  }),
});
