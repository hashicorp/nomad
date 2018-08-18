import { clickable, create, isPresent, text, visitable } from 'ember-cli-page-object';
import { codeFillable, code } from 'nomad-ui/tests/pages/helpers/codemirror';

import error from 'nomad-ui/tests/pages/components/error';

export default create({
  visit: visitable('/jobs/run'),

  planError: error('data-test-plan-error'),
  parseError: error('data-test-parse-error'),
  runError: error('data-test-run-error'),

  plan: clickable('[data-test-plan]'),
  cancel: clickable('[data-test-cancel]'),
  run: clickable('[data-test-run]'),

  planOutput: text('[data-test-plan-output]'),

  planHelp: {
    isPresent: isPresent('[data-test-plan-help-title]'),
    title: text('[data-test-plan-help-title]'),
    message: text('[data-test-plan-help-message]'),
    dismiss: clickable('[data-test-plan-help-dismiss]'),
  },

  editorHelp: {
    isPresent: isPresent('[data-test-editor-help-title]'),
    title: text('[data-test-editor-help-title]'),
    message: text('[data-test-editor-help-message]'),
    dismiss: clickable('[data-test-editor-help-dismiss]'),
  },

  editor: {
    isPresent: isPresent('[data-test-editor]'),
    contents: code('[data-test-editor]'),
    fillIn: codeFillable('[data-test-editor]'),
  },
});
