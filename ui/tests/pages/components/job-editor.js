import { clickable, hasClass, isPresent, text } from 'ember-cli-page-object';
import { codeFillable, code } from 'nomad-ui/tests/pages/helpers/codemirror';

import error from 'nomad-ui/tests/pages/components/error';

export default () => ({
  scope: '[data-test-job-editor]',

  isPresent: isPresent(),

  planError: error('data-test-plan-error'),
  parseError: error('data-test-parse-error'),
  runError: error('data-test-run-error'),

  plan: clickable('[data-test-plan]'),
  cancel: clickable('[data-test-cancel]'),
  run: clickable('[data-test-run]'),

  cancelEditing: clickable('[data-test-cancel-editing]'),
  cancelEditingIsAvailable: isPresent('[data-test-cancel-editing]'),

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

  dryRunMessage: {
    scope: '[data-test-dry-run-message]',
    title: text('[data-test-dry-run-title]'),
    body: text('[data-test-dry-run-body]'),
    errored: hasClass('is-warning'),
    succeeded: hasClass('is-primary'),
  },
});
