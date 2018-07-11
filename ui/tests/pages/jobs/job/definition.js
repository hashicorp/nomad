import { create, isPresent, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/jobs/:id/definition'),

  jsonViewer: isPresent('[data-test-definition-view]'),
});
