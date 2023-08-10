/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  create,
  collection,
  text,
  visitable,
} from 'ember-cli-page-object';
import { getter } from 'ember-cli-page-object/macros';

import twoStepButton from 'nomad-ui/tests/pages/components/two-step-button';
import error from 'nomad-ui/tests/pages/components/error';

export default create({
  visit: visitable('/jobs/:id/versions'),

  versions: collection('[data-test-version]', {
    text: text(),
    stability: text('[data-test-version-stability]'),
    submitTime: text('[data-test-version-submit-time]'),

    revertToButton: twoStepButton('[data-test-revert-to]'),
    revertToButtonIsDisabled: attribute('disabled', '[data-test-revert-to]'),

    number: getter(function () {
      return parseInt(this.text.match(/#(\d+)/)[1]);
    }),
  }),

  error: error(),
});
