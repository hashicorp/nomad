import { create, isPresent, visitable, clickable, text } from 'ember-cli-page-object';

import jobEditor from 'nomad-ui/tests/pages/components/job-editor';

export default create({
  visit: visitable('/jobs/:id/definition'),

  jsonViewer: isPresent('[data-test-definition-view]'),
  editor: jobEditor(),

  edit: clickable('[data-test-edit-job]'),

  error: {
    isPresent: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },
});
