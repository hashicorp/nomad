import { create, isPresent, visitable, clickable } from 'ember-cli-page-object';

import jobEditor from 'nomad-ui/tests/pages/components/job-editor';

export default create({
  visit: visitable('/jobs/:id/definition'),

  jsonViewer: isPresent('[data-test-definition-view]'),
  editor: jobEditor(),

  edit: clickable('[data-test-edit-job]'),
});
